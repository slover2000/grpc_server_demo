package zipkin

import (
	"log"
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/apache/thrift/lib/go/thrift"

	"github.com/slover2000/prisma/trace"
	"github.com/slover2000/prisma/trace/zipkin/thrift-gen/zipkincore"
)

const (
	// Default timeout for http request in seconds
	defaultHTTPTimeout = time.Second * 5

	// defaultBatchInterval in seconds
	defaultHTTPBatchInterval = 1

	defaultHTTPBatchSize = 100

	defaultHTTPMaxBacklog = 1000	
)

// HTTPCollector implements Collector by forwarding spans to a http server.
type HTTPCollector struct {
	url           string
	client        *http.Client
	batchInterval time.Duration
	batchSize     int
	maxBacklog    int
	batch         []*zipkincore.Span
	spanc         chan *trace.Span
	quit          chan struct{}
	shutdown      chan error
	sendMutex     *sync.Mutex
	batchMutex    *sync.Mutex
}

// HTTPOption sets a parameter for the HttpCollector
type HTTPOption func(c *HTTPCollector)

// HTTPTimeout sets maximum timeout for http request.
func HTTPTimeout(duration time.Duration) HTTPOption {
	return func(c *HTTPCollector) { c.client.Timeout = duration }
}

// HTTPBatchSize sets the maximum batch size, after which a collect will be
// triggered. The default batch size is 100 traces.
func HTTPBatchSize(n int) HTTPOption {
	return func(c *HTTPCollector) { c.batchSize = n }
}

// HTTPMaxBacklog sets the maximum backlog size,
// when batch size reaches this threshold, spans from the
// beginning of the batch will be disposed
func HTTPMaxBacklog(n int) HTTPOption {
	return func(c *HTTPCollector) { c.maxBacklog = n }
}

// HTTPBatchInterval sets the maximum duration we will buffer traces before
// emitting them to the collector. The default batch interval is 1 second.
func HTTPBatchInterval(d time.Duration) HTTPOption {
	return func(c *HTTPCollector) { c.batchInterval = d }
}

// HTTPClient sets a custom http client to use.
func HTTPClient(client *http.Client) HTTPOption {
	return func(c *HTTPCollector) { c.client = client }
}

// NewHTTPCollector returns a new HTTP-backend Collector. url should be a http
// url for handle post request, the url like "http://ip:port/api/v1/spans". timeout is passed to http client. queueSize control
// the maximum size of buffer of async queue. The logger is used to log errors,
// such as send failures;
func NewHTTPCollector(url string, options ...HTTPOption) trace.Collector {
	c := &HTTPCollector{
		url:           url,
		client:        &http.Client{Timeout: defaultHTTPTimeout},
		batchInterval: defaultHTTPBatchInterval * time.Second,
		batchSize:     defaultHTTPBatchSize,
		maxBacklog:    defaultHTTPMaxBacklog,
		batch:         []*zipkincore.Span{},
		spanc:         make(chan *trace.Span),
		quit:          make(chan struct{}, 1),
		shutdown:      make(chan error, 1),
		sendMutex:     &sync.Mutex{},
		batchMutex:    &sync.Mutex{},
	}

	for _, option := range options {
		option(c)
	}

	go c.loop()
	return c
}

// Collect implements Collector.
func (c *HTTPCollector) Collect(s *trace.Span) error {
	c.spanc <- s
	return nil
}

// Close implements Collector.
func (c *HTTPCollector) Close() error {
	close(c.quit)
	return <-c.shutdown
}

func httpSerialize(spans []*zipkincore.Span) *bytes.Buffer {
	t := thrift.NewTMemoryBuffer()
	p := thrift.NewTBinaryProtocolTransport(t)
	if err := p.WriteListBegin(thrift.STRUCT, len(spans)); err != nil {
		log.Printf("write span lenght start failed:%s", err.Error())
		return nil
	}
	for _, s := range spans {
		if err := s.Write(p); err != nil {
			log.Printf("write span data[%s] failed:%s", s.GetName(), err.Error())
			return nil
		}
	}
	if err := p.WriteListEnd(); err != nil {
		log.Printf("write span lenght end failed:%s", err.Error())
		return nil
	}
	return t.Buffer
}

func (c *HTTPCollector) loop() {
	var (
		nextSend = time.Now().Add(c.batchInterval)
		ticker   = time.NewTicker(c.batchInterval / 10)
		tickc    = ticker.C
	)
	defer ticker.Stop()

	for {
		select {
		case span := <-c.spanc:
			zpSpan, err := ConvertToZipkinSpan(span)
			if err == nil {
				currentBatchSize := c.append(zpSpan)
				if currentBatchSize >= c.batchSize {
					nextSend = time.Now().Add(c.batchInterval)
					go c.send()
				}
			} else {
				log.Printf("convert span to zipkin format failed:%s", err.Error())
			}
		case <-tickc:
			if time.Now().After(nextSend) {
				nextSend = time.Now().Add(c.batchInterval)
				go c.send()
			}
		case <-c.quit:
			c.shutdown <- c.send()
			return
		}
	}
}

func (c *HTTPCollector) append(span *zipkincore.Span) (newBatchSize int) {
	c.batchMutex.Lock()
	defer c.batchMutex.Unlock()

	c.batch = append(c.batch, span)
	if len(c.batch) > c.maxBacklog {
		dispose := len(c.batch) - c.maxBacklog
		c.batch = c.batch[dispose:]
	}
	newBatchSize = len(c.batch)
	return
}

func (c *HTTPCollector) send() error {
	// in order to prevent sending the same batch twice
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	// Select all current spans in the batch to be sent
	c.batchMutex.Lock()
	sendBatch := c.batch[:]
	c.batchMutex.Unlock()

	// Do not send an empty batch
	if len(sendBatch) == 0 {
		return nil
	}

	req, err := http.NewRequest(
		"POST",
		c.url,
		httpSerialize(sendBatch))
	if err != nil {
		log.Printf("err:%s", err.Error())
		return err
	}
	req.Header.Set("Content-Type", "application/x-thrift")
	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("err:%s", err.Error())
		return err
	}
	resp.Body.Close()
	// non 2xx code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("HTTP POST span failed code:%s", resp.Status)
	}

	// Remove sent spans from the batch
	c.batchMutex.Lock()
	c.batch = c.batch[len(sendBatch):]
	c.batchMutex.Unlock()

	return nil
}