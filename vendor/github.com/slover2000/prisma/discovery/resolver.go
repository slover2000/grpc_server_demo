package discovery

import (
	"log"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"encoding/json"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"google.golang.org/grpc/naming"
)


// resolver is the implementaion of grpc.naming.Resolver
type etcdResolver struct {
	systemName	string
	serviceName string
	environment EnvironmentType
	groupName		string
	dialTimeout time.Duration
	client      *clientv3.Client
	closed 			bool
}

type ResolverOption func(*etcdResolver)

func WithResolverSystem(name string) ResolverOption {
	return func(r *etcdResolver) { r.systemName = name }
}

func WithResolverService(name string) ResolverOption {
	return func(r *etcdResolver) { r.serviceName = name }
}

func WithEnvironment(t EnvironmentType) ResolverOption {
	return func(r *etcdResolver) { r.environment = t }
}

func WithResolverGroup(name string) ResolverOption {
	return func(r *etcdResolver) { r.groupName = name }
}

func WithDialTimeout(timeout time.Duration) ResolverOption {
	return func(r *etcdResolver) { r.dialTimeout = timeout }
}

// etcdWatcher is the implementaion of grpc.naming.Watcher
type etcdWatcher struct {
	resolver      *etcdResolver
	isInitialized bool
}

// NewResolver return resolver with service name
func NewEtcdResolver(options ...ResolverOption) *etcdResolver {
	resolver := &etcdResolver{
		systemName: GRPCSystem,
		environment: Product,
		dialTimeout: 5 * time.Second,
	}

	for _, option := range options {
		option(resolver)
	}

	return resolver
}

type Operation = string
const (
	Add		= "add"
	Delete 	= "del"
)

type Update struct {
	Op		Operation
	Addr	string
	Meta 	interface{}
}

type ChangeReceiver func([]*Update)

func (r *etcdResolver) Watch(target string, receiver ChangeReceiver) ([]*Update, error) {
	if len(r.serviceName) == 0 {
		return nil, errors.New("no service name provided")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(target, ","),
		DialTimeout: r.dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	r.client = client

	prefix := fmt.Sprintf("/%s/%s/%s", r.systemName, r.serviceName, EnviormentTypeToString(r.environment))
	// query addresses from etcd
	ctx, cancel := context.WithTimeout(context.Background(), r.dialTimeout)
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	cancel()
	if err == nil {
		updates := []*Update{}
		addrs := extractAddrs(resp, r.groupName)
		//if not empty, return the updates
		if l := len(addrs); l != 0 {
			updates := make([]*Update, l)
			for i, ep := range addrs {
				addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
				updates[i] = &Update{Op: Add, Addr: addr, Meta: &ep}
			}
		}

		if receiver != nil {
			// start goroutine to watch
			go func(c *etcdResolver, path string, r ChangeReceiver) {
				// watch the path
				for {
					rch := c.client.Watch(context.Background(), path, clientv3.WithPrefix())
					for wresp := range rch {
						for _, ev := range wresp.Events {
							ep := decodeEndpointFromEvent(ev)
							addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
							switch ev.Type {
							case mvccpb.PUT:
								r([]*Update{{Op: Add, Addr: addr, Meta: &ep}})
							case mvccpb.DELETE:
								r([]*Update{{Op: Delete, Addr: addr, Meta: &ep}})
							}
						}
					}
					if c.closed {
						return
					}
				}
			}(r, prefix, receiver)
		}

		return updates, nil
	} else {
		log.Printf("etcd resolver: get key with prefix[%s] failed:%s", prefix, err.Error())
		return nil, err
	}
}

// Close close etcd v3 client
func (r *etcdResolver) Close() {
	r.client.Close()
	r.closed = true
}

// @Title resolve the service from etcd, target is the dial address of etcd 
// @Description create a watcher of etcdv3
// @Param   key target    string  true    "the etcd address, like http://127.0.0.1:2379,http://127.0.0.1:12379,http://127.0.0.1:22379"
// @Success return an instance of watcher
// @Failure return error of etcd3
func (r *etcdResolver) Resolve(target string) (naming.Watcher, error) {
	if len(r.serviceName) == 0 {
		return nil, errors.New("no service name provided")
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   strings.Split(target, ","),
		DialTimeout: r.dialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("creat etcd3 client failed: %s", err.Error())
	}
	r.client = client
	return &etcdWatcher{resolver: r}, nil
}

// Close close etcd v3 client
func (w *etcdWatcher) Close() {
	w.resolver.client.Close()
}

// Next to return the updates
func (w *etcdWatcher) Next() ([]*naming.Update, error) {
	// prefix is the etcd prefix/value to watch
	prefix := fmt.Sprintf("/%s/%s/%s", w.resolver.systemName, w.resolver.serviceName, EnviormentTypeToString(w.resolver.environment))
	// check if is initialized
	if !w.isInitialized {
		// query addresses from etcd
		ctx, cancel := context.WithTimeout(context.Background(), w.resolver.dialTimeout)
		resp, err := w.resolver.client.Get(ctx, prefix, clientv3.WithPrefix())
		cancel()
		if err == nil {
			w.isInitialized = true
			addrs := extractAddrs(resp, w.resolver.groupName)
			//if not empty, return the updates or watcher new dir
			if l := len(addrs); l != 0 {
				updates := make([]*naming.Update, l)
				for i, ep := range addrs {
					addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
					updates[i] = &naming.Update{Op: naming.Add, Addr: addr}
				}
				return updates, nil
			}
		} else {
			log.Printf("get key with prefix[%s] failed:%s", prefix, err.Error())
		}
	}
	// generate etcd Watcher
	rch := w.resolver.client.Watch(context.Background(), prefix, clientv3.WithPrefix())
	for wresp := range rch {
		for _, ev := range wresp.Events {
			ep := decodeEndpointFromEvent(ev)
			addr := fmt.Sprintf("%s:%d", ep.Host, ep.Port)
			switch ev.Type {
			case mvccpb.PUT:
				return []*naming.Update{{Op: naming.Add, Addr: addr}}, nil
			case mvccpb.DELETE:
				return []*naming.Update{{Op: naming.Delete, Addr: addr}}, nil
			}
		}
	}
	return nil, nil
}

func extractAddrs(resp *clientv3.GetResponse, group string) []Endpoint {
	addrs := make([]Endpoint, 0)
	if resp == nil || resp.Kvs == nil {
		return addrs
	}

	for i := range resp.Kvs {
		if v := resp.Kvs[i].Value; v != nil {
			ep := &Endpoint{}
			json.Unmarshal(v, ep)
			if len(ep.Group) == 0 || ep.Group == DefaultGroup {
				addrs = append(addrs, *ep)
			} else if len(group) > 0 && group != DefaultGroup {
				if group == ep.Group {
					addrs = append(addrs, *ep)
				}
			}
		}
	}
	return addrs
}

func decodeEndpointFromEvent(ev *clientv3.Event) *Endpoint {
	ep := &Endpoint{}
	json.Unmarshal(ev.Kv.Value, ep)
	return ep
}
