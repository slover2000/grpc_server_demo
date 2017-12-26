package discovery

import (
	"fmt"
    "log"
    "strings"
    "time"
    "errors"
    "context"
	"encoding/json"
	
	"github.com/coreos/etcd/clientv3"
)

type registerOptions struct {
    systemName  string
    serviceName string
    endpoint    Endpoint
    dialTimeout time.Duration
    interval    time.Duration
    ttl         time.Duration
}

type RegisterOption func(*registerOptions)

func WithRegisterSystem(name string) RegisterOption {
    return func(c *registerOptions) { c.systemName = name }
}

func WithRegisterService(name string) RegisterOption {
    return func(c *registerOptions) { c.serviceName = name }
}

func WithRegisterEndpoint(ep Endpoint) RegisterOption {
    return func(c *registerOptions) { c.endpoint = ep }
}

func WithRegisterDialTimeout(timeout time.Duration) RegisterOption {
    return func(c *registerOptions) { c.dialTimeout = timeout }
}

func WithRegisterInterval(interval time.Duration) RegisterOption {
    return func(c *registerOptions) { c.interval = interval }
}

func WithRegisterTTL(ttl time.Duration) RegisterOption {
    return func(c *registerOptions) { c.ttl = ttl }
}

type EtcdRegister interface {
    Register() error
    Unregister() error
}

type etcdRegister struct {
    client      *clientv3.Client
    quit        chan bool
    options     registerOptions
}

func NewEtcdRegister(etcdAddr string, options ...RegisterOption) (EtcdRegister, error) {    
    register := &etcdRegister{
        quit:   make(chan bool, 1),
        options: registerOptions{
            systemName: GRPCSystem,
            dialTimeout:5 * time.Second,
            interval:   10 * time.Second,
            ttl:        15 * time.Second,
        },
    }

	for _, option := range options {
		option(&register.options)
    }

    // precheck endpoint properties
    if len(register.options.endpoint.Host) == 0 {
        return nil, errors.New("endpoint must have a host.")
    }

    if register.options.endpoint.Port == 0 {
        return nil, errors.New("endpoint must have a port.")
    }

    if len(register.options.serviceName) == 0 {
        return nil, errors.New("must provide a service name.")
    }
    initEndpointWithDefault(&register.options.endpoint)
    
	client, err := clientv3.New(clientv3.Config{
		Endpoints: strings.Split(etcdAddr, ","),
		DialTimeout: register.options.dialTimeout,
    })
    
    if err != nil {
        return nil, fmt.Errorf("create etcd3 client failed: %v", err)
    }

    register.client = client
    return register, nil
}

func (r *etcdRegister) Register() error {
	go func(c *etcdRegister) {
        addressValue := fmt.Sprintf("%s:%d", c.options.endpoint.Host, c.options.endpoint.Port)
        serviceKey = fmt.Sprintf("/%s/%s/%s/%s", c.options.systemName, c.options.serviceName, EnviormentTypeToString(c.options.endpoint.EnvType), addressValue)
        endpointValue, _ := json.Marshal(&c.options.endpoint)

        // delete any previous existent service node
        c.client.Delete(context.Background(), serviceKey)

        // minimum lease TTL is ttl-second
        resp, _ := c.client.Grant(context.TODO(), int64(c.options.ttl.Seconds()))
        leaseID := resp.ID
        ctx, cancel := context.WithTimeout(context.Background(), c.options.dialTimeout)
        _, err := c.client.Put(ctx, serviceKey, string(endpointValue), clientv3.WithLease(leaseID))
        cancel()
        if err != nil {
            log.Fatalf("register service '%s' with ttl to etcd3 failed: %s", c.options.serviceName, err.Error())
            return
        }

        // invoke self-refresh with ticker
        ticker := time.NewTicker(c.options.interval)
        for {    
            select {
            case <-c.quit:
                return
            case <-ticker.C:
                // refresh set to true for not notifying the watcher
                ctx, cancel := context.WithTimeout(context.Background(), c.options.dialTimeout)
                _, err = c.client.KeepAliveOnce(ctx, leaseID)
                cancel()
                if err != nil {
                    log.Printf("refresh service '%s' with ttl to etcd3 failed: %s", c.options.serviceName, err.Error())
                    // reset the key
                    resp, _ := c.client.Grant(context.TODO(), int64(c.options.ttl.Seconds()))
                    leaseID = resp.ID
                    ctx, cancel = context.WithTimeout(context.Background(), c.options.dialTimeout)
                    _, err = c.client.Put(ctx, serviceKey, string(endpointValue), clientv3.WithLease(leaseID))
                    cancel()
                    if err != nil {
                        log.Fatalf("reregister service '%s' with ttl to etcd3 failed: %s", c.options.serviceName, err.Error())
                    } else {
                        log.Printf("reset service '%s' to etcd3 successs", c.options.serviceName,)
                    }
                }
            }
        }
    }(r)

    return nil
}

func (r *etcdRegister) Unregister() error {
    r.quit <- true
    close(r.quit)
    defer r.client.Close()

    addressValue := fmt.Sprintf("%s:%d", r.options.endpoint.Host, r.options.endpoint.Port)
    serviceKey = fmt.Sprintf("/%s/%s/%s/%s", r.options.systemName, r.options.serviceName, EnviormentTypeToString(r.options.endpoint.EnvType), addressValue)
    _, err := r.client.Delete(context.Background(), serviceKey)
    if err != nil {
        log.Printf("unregister '%s' failed with: %s", serviceKey, err.Error())
    } else {
        log.Printf("unregister '%s' success", serviceKey)
    }
    
    return err
}

var stopSignal = make(chan bool, 1)
var client clientv3.Client
var serviceKey string

func initEndpointWithDefault(ep *Endpoint) {
    if ep.Weight == 0 {
        ep.Weight = DefaultWeight
    }
}

// Register register a service into etcdv3
// @Param target string the ectdv3 address
func RegisterWithEtcd(serviceName string, target string, ep Endpoint, interval, ttl time.Duration) error {
    if len(ep.Host) == 0 {
        return errors.New("endpoint must have a host.")
    }

    if ep.Port == 0 {
        return errors.New("endpoint must have a port.")
    }
    initEndpointWithDefault(&ep)

	addressValue := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
	serviceKey = fmt.Sprintf("/%s/%s/%s/%s", GRPCSystem, serviceName, EnviormentTypeToString(ep.EnvType), addressValue)
    endpointValue, _ := json.Marshal(&ep)

	// get endpoints for register dial address
	client, err := clientv3.New(clientv3.Config{
		Endpoints: strings.Split(target, ","),
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
        return fmt.Errorf("create etcd3 client failed: %v", err)
    }

	go func() {
        // delete any previous existent service node
        client.Delete(context.Background(), serviceKey)

        // minimum lease TTL is ttl-second
        resp, _ := client.Grant(context.TODO(), int64(ttl.Seconds()))
        leaseID := resp.ID
        ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout * time.Second)
        _, err = client.Put(ctx, serviceKey, string(endpointValue), clientv3.WithLease(leaseID))
        cancel()
        if err != nil {
            log.Fatalf("register service '%s' with ttl to etcd3 failed: %s", serviceName, err.Error())
            return
        }

        // invoke self-refresh with ticker
        ticker := time.NewTicker(interval)
        for {    
            select {
            case <-stopSignal:
                return
            case <-ticker.C:
                // refresh set to true for not notifying the watcher
                ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout * time.Second)
                _, err = client.KeepAliveOnce(ctx, leaseID)
                cancel()
                if err != nil {
                    log.Printf("refresh service '%s' with ttl to etcd3 failed: %s", serviceName, err.Error())
                }
            }
        }
    }()

    return nil	
}

// UnRegister delete registered service from etcd
func UnRegisterWithEtcd() error {
    stopSignal <- true
    close(stopSignal)
    defer client.Close()

    var err error
    if _, err = client.Delete(context.Background(), serviceKey); err != nil {
        log.Printf("deregister '%s' failed: %s", serviceKey, err.Error())
    } else {
        log.Printf("deregister '%s' ok.", serviceKey)
    }
    
    return err
}