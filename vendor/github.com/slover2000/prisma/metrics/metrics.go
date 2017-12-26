package metrics

import (
	"fmt"
	"log"
	"time"
	"strings"
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	p "github.com/slover2000/prisma/thirdparty"
)

type grpcType = string
const (
	Unary        grpcType = "unary"
	ClientStream grpcType = "client_stream"
	ServerStream grpcType = "server_stream"
	BidiStream   grpcType = "bidi_stream"

	defaultListenPort	 = 9090
	defaultMetricsPath	 = "/metrics"
)

// ClientMetrics represent a metrics client
type ClientMetrics interface {
	CounterGRPC(name string, duration time.Duration, err error)
	CounterHTTP(req *http.Request, duration time.Duration, code int)
	CounterDatabase(params *p.DatabaseParam, duration time.Duration, err error)
	CounterCache(params *p.CacheParam, duration time.Duration, err error)
	CounterSearch(params *p.SearchParam, duration time.Duration, err error)
	Close() error
}

type MetricsHttpServer interface {
	Shutdown() error
}

type metricsPrometheusMetricsHTTPServer struct {
	port 	int
	server *http.Server
}

func SplitGRPCMethodName(fullMethodName string) (string, string) {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	if i := strings.Index(fullMethodName, "/"); i >= 0 {
		return fullMethodName[:i], fullMethodName[i+1:]
	}
	return "unknown", "unknown"
}

func ConvertMethodName(fullMethodName string) string {
	fullMethodName = strings.TrimPrefix(fullMethodName, "/") // remove leading slash
	return strings.Replace(fullMethodName, "/", ".", -1)
}

func StartPrometheusMetricsHTTPServer(port int) MetricsHttpServer {
	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	if port == 0 {
		port = defaultListenPort
	}

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}
	http.Handle(defaultMetricsPath, promhttp.Handler())

	client := &metricsPrometheusMetricsHTTPServer{
		port: port,
		server: srv,
	}
    go func() {
		log.Printf("Http metrics server: listen on port:%d", port)
        if err := srv.ListenAndServe(); err != nil {
            // cannot panic, because this probably is an intentional close
            log.Printf("Http metrics server: ListenAndServe() error: %s", err)
        }
	}()
	
	return client
}

func (s *metricsPrometheusMetricsHTTPServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	err := s.server.Shutdown(ctx)
	cancel()
	return err
}