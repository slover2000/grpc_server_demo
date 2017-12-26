package perfcounter

import (
	"fmt"
	"time"
	"net/http"

	"github.com/rcrowley/go-metrics"

	m "github.com/slover2000/prisma/metrics"
	p "github.com/slover2000/prisma/thirdparty"
)

const (
	defaultName 		= "default"
	meterSuffixName 	= ".METER"
	counterSuffixName	= ".COUNTER"
	histogramSuffixName	= ".HIST"
	failedSuffixName	= ".fail"

	defaultProjectName	= "default_project"
)

type perfcounterClient struct {
	project    string
	registry   metrics.Registry
}

// NewPerfcounter return a new perfcounter client
func NewPerfcounter(project string) m.ClientMetrics {
	client := &perfcounterClient{
		project: project,
		registry: metrics.NewRegistry(),
	}
	return client
}

func (c *perfcounterClient) CounterGRPC(name string, duration time.Duration, err error) {
	if c == nil {
		return
	}

	// 记录Counter, like QPS
	c.count(name, 1, err)

	// 记录Histogram, in millisecond, measure cost time of every method
	c.countHistogram(name, duration, err)
}

func (c *perfcounterClient) CounterHTTP(req *http.Request, duration time.Duration, code int) {
	if c == nil {
		return
	}

	name := req.URL.Path // drop scheme and query params

	// 记录Counter, like QPS
	if code == http.StatusOK {
		c.count(name, 1, nil)
	} else {
		c.count(name, 1, fmt.Errorf("status code: %d", code))
	}
	
	// 记录Histogram, in millisecond, measure cost time of every method
	if code == http.StatusOK {
		c.countHistogram(name, duration, nil)
	} else {
		c.countHistogram(name, duration, fmt.Errorf("status code: %d", code))
	}	
}

func (c *perfcounterClient) CounterDatabase(params *p.DatabaseParam, duration time.Duration, err error) {

}

func (c *perfcounterClient) CounterCache(params *p.CacheParam, duration time.Duration, err error) {
	
}

func (c *perfcounterClient) CounterSearch(params *p.SearchParam, duration time.Duration, err error) {
	
}

func (c *perfcounterClient) count(name string, count int64, err error) {
	realName := makeMetricsName(name, meterSuffixName, err)
	meter := metrics.GetOrRegisterMeter(realName, c.registry)
	meter.Mark(count)
}

func (c *perfcounterClient) countHistogram(name string, duration time.Duration, err error) {
	realName := makeMetricsName(name, histogramSuffixName, err)
	histogram := metrics.GetOrRegisterHistogram(realName, c.registry, metrics.NewExpDecaySample(1028, 0.015))
	milliscond := duration.Nanoseconds() / int64(time.Millisecond)
	if milliscond == 0 {
		milliscond = 1
	}
	histogram.Update(milliscond)
}

func (c *perfcounterClient) Close() error {
	return nil
}

func makeMetricsName(name, suffix string, err error) string {
	realName := m.ConvertMethodName(name)
	if len(name) == 0 {
		realName = defaultName
	}

	if err != nil {
		return fmt.Sprintf("%s%s%s", realName, failedSuffixName, suffix)
	}
	return fmt.Sprintf("%s%s", realName, suffix)
}