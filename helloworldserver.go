package main

import (
    "flag"
    "fmt"
    "log"
    "net"
    "os"
    "io"
    "os/signal"
    "syscall"
    "time"
    "net/http"
    
    "github.com/sirupsen/logrus"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    
    "github.com/slover2000/prisma"
    "github.com/slover2000/prisma/logging"
    "github.com/slover2000/prisma/discovery"
    "github.com/slover2000/prisma/trace"
    "github.com/slover2000/prisma/trace/zipkin"
    pb "51nap.com/grpc_demo/helloworld"
)

var (
    serv = flag.String("service", "hello_service", "service name")
    port = flag.Int("port", 50001, "listening port")
    reg = flag.String("reg", "http://10.98.16.215:2379", "register etcd address")
    zphost = flag.String("zipkin", "http://10.98.16.215:9411/api/v1/spans", "zipkin server address")
)

func main() {
    // initialize logger
    logger := logrus.New()
    logger.Formatter = &logrus.JSONFormatter{}
    // Use logrus for standard log output
    // Note that `log` here references stdlib's log
    // Not logrus imported under the name `log`.
    log.SetFlags(0)
    log.SetOutput(logger.Writer()) 

    flag.Parse()
    lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
    if err != nil {
        panic(err)
    }
    
    // create interceptor for grpc server
    serviceName := "scraper_service"
    policy, _ := trace.NewLimitedSampler(0.1, 10)
    collector := trace.NewMultiCollector(
                    trace.NewConsoleCollector(), 
                    zipkin.NewHTTPCollector(*zphost, zipkin.HTTPBatchSize(10), zipkin.HTTPMaxBacklog(3), zipkin.HTTPBatchInterval(3 * time.Second)))
    interceptorClient, err := prisma.ConfigInterceptorClient(
            context.Background(),            
            prisma.EnableTracing(serviceName, policy, collector),
            prisma.EnableLoggingWithEntry(logging.InfoLevel, logrus.NewEntry(logger)),
            prisma.EnableGRPCServerMetrics(),
            prisma.EnableHTTPClientMetrics(),
            prisma.EnableMetricsExportHTTPServer(9090))
    if err != nil {
        log.Printf("create interceptor failed:%s", err.Error())
        return
    }
    
    // register server into etcd
    endpoint := discovery.Endpoint{Host: "127.0.0.1", Port: *port, EnvType: discovery.Product}
    register, err := discovery.NewEtcdRegister(
                    *reg,
                    discovery.WithRegisterSystem(discovery.GRPCSystem),
                    discovery.WithRegisterService(*serv),
                    discovery.WithRegisterEndpoint(endpoint),
                    discovery.WithRegisterDialTimeout(5 * time.Second),
                    discovery.WithRegisterInterval(100 * time.Second),
                    discovery.WithRegisterTTL(150 * time.Second))
    if err != nil {
        panic(err)
    }
    register.Register()

    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL, syscall.SIGHUP, syscall.SIGQUIT)
    go func() {
        s := <-ch
        log.Printf("receive signal '%v'", s)
        register.Unregister()
        interceptorClient.Close()
        os.Exit(1)
    }()

    log.Printf("starting hello service on %d", *port)
    s := grpc.NewServer(grpc.UnaryInterceptor(interceptorClient.GRPCUnaryServerInterceptor()), grpc.StreamInterceptor(interceptorClient.GRPStreamServerInterceptor()))
    pb.RegisterGreeterServer(s, &server{client: interceptorClient})
    s.Serve(lis)
}

// server is used to implement helloworld.GreeterServer.
type server struct{
    client *prisma.InterceptorClient
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
    fmt.Printf("%v: Receive is %s\n", time.Now(), in.Name)
    exampleHTTPClientDo(ctx, s.client)
    return &pb.HelloReply{Message: "Hello " + in.Name}, nil
}

func (s *server) SayStreamHello(gs pb.Greeter_SayStreamHelloServer) error {
    for {
		in, err := gs.Recv()
		if err == io.EOF || codes.Canceled == grpc.Code(err) {
			return nil
        }
		if err != nil {
			log.Printf("failed to recv stream: %v", err)
			return err
        }
        fmt.Printf("%v: Receive stream is %s\n", time.Now(), in.Name)
        gs.Send(&pb.HelloReply{Message: "Hello stream " + in.Name})
	}
}

func exampleHTTPClientDo(ctx context.Context, c *prisma.InterceptorClient) {
    client := http.Client{
        Transport: &prisma.Transport{Client: c},
    }
    req, _ := http.NewRequest("GET", "http://www.baidu.com", nil)
    req = req.WithContext(ctx)

    if _, err := client.Do(req); err != nil {
        log.Fatal(err)
    }
} 