package prometheus

import (
	"time"
	"fmt"
	"net/http"

	"google.golang.org/grpc"

	m "github.com/slover2000/prisma/metrics"
	p "github.com/slover2000/prisma/thirdparty"
	prom "github.com/prometheus/client_golang/prometheus"	
)

const (
	defaultProjectName	= "default_project"
	defaultReportInterval = 60
)

// defaultBuckets are the default Histogram buckets. The default buckets are
// tailored to broadly measure the response time (in millisecond) of a network
// service. Most likely, however, you will be required to define buckets
// customized to your use case.
var (
	defaultBuckets = []float64{1.0, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500., 1000.0, 2500.0, 5000.0, 10000.0}
)

type prometheusClient struct {
	totalCounter       *prom.CounterVec
	errorCounter       *prom.CounterVec		
	durationHistogram  *prom.HistogramVec
	properties 			map[string]string
}

// NewGRPCClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCClientPrometheus(buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "grpc_request_total",
				Help: "Total number of RPCs completed by the client, regardless of success or failure.",
			}, []string{"service", "method", "code"}),
			
		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "grpc_request_failures_total",
				Help: "Total number of RPCs failed by the client.",
			}, []string{"service", "method", "code"}),
		
		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: "grpc_request_duration_ms",
				Help: "Histogram of response latency (milliseconds) of the gRPC until it is finished by the application.",
				Buckets: buckets,
			},
			[]string{"service", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewGRPCServerPrometheus returns a ServerMetrics object. Use a new instance of
// ServerMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewGRPCServerPrometheus(buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "grpc_handled_total",
				Help: "Total number of RPCs completed on the server, regardless of success or failure.",
			}, []string{"service", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "grpc_handled_failures_total",
				Help: "Total number of RPCs failed by the client.",
			}, []string{"service", "method", "code"}),			

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: "grpc_handled_duration_ms",
				Help: "Histogram of response latency (seconds) of gRPC that had been application-level handled by the server.",
				Buckets: buckets,
			},
			[]string{"service", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewHTTPClientPrometheus returns a ServerMetrics object.
func NewHTTPClientPrometheus(buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "http_request_total",
				Help: "Total number of http completed by the client, regardless of success or failure.",
			}, []string{"domain", "path", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "http_request_failures_total",
				Help: "Total number of http failed by the client.",
			}, []string{"domain", "path", "method", "code"}),

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: "http_request_duration_ms",
				Help: "Histogram of response latency (milliseconds) of the http until it is finished by the application.",
				Buckets: buckets,
			},
			[]string{ "domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewHTTPServerPrometheus returns a ServerMetrics object.
func NewHTTPServerPrometheus(buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "http_handled_total",
				Help: "Total number of http completed by the server, regardless of success or failure.",
			}, []string{"domain", "path", "method", "code"}),

		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: "http_handled_failures_total",
				Help: "Total number of http failed by the server.",
			}, []string{"domain", "path", "method", "code"}),			

		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: "http_handled_duration_ms",
				Help: "Histogram of response latency (milliseconds) of the http until it is finished by the application.",
				Buckets: buckets,
			},
			[]string{"domain", "path", "method", "code"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewDatabaseClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewDatabaseClientPrometheus(system string, buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_total", system),
				Help: fmt.Sprintf("Total number of request completed by the %s, regardless of success or failure.", system),
			}, []string{"db", "table", "method", "error"}),
			
		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_failures_total", system),
				Help: fmt.Sprintf("Total number of request failed by the %s.", system),
			}, []string{"db", "table", "method", "error"}),
		
		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_request_duration_ms", system),
				Help: fmt.Sprintf("Histogram of response latency (milliseconds) of the request until it is finished by %s.", system),
				Buckets: buckets,
			},
			[]string{"db", "table", "method", "error"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewCacheClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewCacheClientPrometheus(system string, buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_total", system),
				Help: fmt.Sprintf("Total number of request completed by the %s, regardless of success or failure.", system),
			}, []string{"method", "error"}),
			
		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_failures_total", system),
				Help: fmt.Sprintf("Total number of request failed by the %s.", system),
			}, []string{"method", "error"}),
		
		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_request_duration_ms", system),
				Help: fmt.Sprintf("Histogram of response latency (milliseconds) of the request until it is finished by %s.", system),
				Buckets: buckets,
			},
			[]string{"method", "error"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

// NewSearchClientPrometheus returns a ClientMetrics object. Use a new instance of
// ClientMetrics when not using the default Prometheus metrics registry, for
// example when wanting to control which metrics are added to a registry as
// opposed to automatically adding metrics via init functions.
func NewSearchClientPrometheus(system string, buckets []float64) m.ClientMetrics {
	if len(buckets) == 0 {
		buckets = defaultBuckets
	}

	client := &prometheusClient{
		totalCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_total", system),
				Help: fmt.Sprintf("Total number of request completed by the %s, regardless of success or failure.", system),
			}, []string{"index", "document", "method", "error"}),
			
		errorCounter: prom.NewCounterVec(
			prom.CounterOpts{
				Name: fmt.Sprintf("%s_request_failures_total", system),
				Help: fmt.Sprintf("Total number of request failed by the %s.", system),
			}, []string{"index", "document", "method", "error"}),
		
		durationHistogram: prom.NewHistogramVec(
			prom.HistogramOpts{
				Name: fmt.Sprintf("%s_request_duration_ms", system),
				Help: fmt.Sprintf("Histogram of response latency (milliseconds) of the request until it is finished by %s.", system),
				Buckets: buckets,
			},
			[]string{"index", "document", "method", "error"},
		),
	}
	prom.MustRegister(client.totalCounter)
	prom.MustRegister(client.errorCounter)
	prom.MustRegister(client.durationHistogram)

	return client
}

func (c *prometheusClient) CounterGRPC(name string, duration time.Duration, err error) {
	if c == nil {
		return
	}

	code := grpc.Code(err)
	serviceName, methodName := m.SplitGRPCMethodName(name)
	// 记录total counter
	c.totalCounter.WithLabelValues(serviceName, methodName, code.String()).Inc()

	// 记录failurs counter
	if err != nil {
		c.errorCounter.WithLabelValues(serviceName, methodName, code.String()).Inc()
	}

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(serviceName, methodName, code.String()).Observe(float64(ms))
}

func (c *prometheusClient) CounterHTTP(req *http.Request, duration time.Duration, code int) {
	if c == nil {
		return
	}
		
	// 记录total counter, like QPS
	c.totalCounter.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Inc()

	// 记录failurs counter
	if code >= 400 {
		c.errorCounter.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Inc()
	}	

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(req.URL.Host, req.URL.Path, req.Method, fmt.Sprintf("%d", code)).Observe(float64(ms))
}


func (c *prometheusClient) CounterDatabase(params *p.DatabaseParam, duration time.Duration, err error) {
	if c == nil {
		return
	}
		
	// 记录total counter, like QPS
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	c.totalCounter.WithLabelValues(params.Database, params.Table, params.Action, errStr).Inc()

	// 记录failurs counter
	if err != nil {
		c.errorCounter.WithLabelValues(params.Database, params.Table, params.Action, errStr).Inc()
	}

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(params.Database, params.Table, params.Action, errStr).Observe(float64(ms))
}

func (c *prometheusClient) Close() error {
	return nil
}

func (c *prometheusClient) CounterCache(params *p.CacheParam, duration time.Duration, err error) {
	if c == nil {
		return
	}

	// 记录total counter, like QPS
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	c.totalCounter.WithLabelValues(params.Action, errStr).Inc()

	// 记录failurs counter
	if err != nil {
		c.errorCounter.WithLabelValues(params.Action, errStr).Inc()
	}

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(params.Action, errStr).Observe(float64(ms))
}

func (c *prometheusClient) CounterSearch(params *p.SearchParam, duration time.Duration, err error) {
	if c == nil {
		return
	}
	
	// 记录total counter, like QPS
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	c.totalCounter.WithLabelValues(params.Index, params.Document, params.Action, errStr).Inc()

	// 记录failurs counter
	if err != nil {
		c.errorCounter.WithLabelValues(params.Index, params.Document, params.Action, errStr).Inc()
	}

	// 记录Histogram, in millisecond, measure cost time of every method
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	c.durationHistogram.WithLabelValues(params.Index, params.Document, params.Action, errStr).Observe(float64(ms))	
}