package trace

import (
	"fmt"
	"log"
)

// Collector represents a trace collector, which is probably a set of
// remote endpoints.
type Collector interface {
	Collect(*Span) error
	Close() error
}

// NopCollector implements Collector but performs no work.
type NopCollector struct{}

// Collect implements Collector.
func (*NopCollector) Collect(*Span) error { return nil }

// Close implements Collector.
func (*NopCollector) Close() error { return nil }

func NewNopCollector() Collector {
	return &NopCollector{}
}

type LogCollector struct {
	encoder TraceEncoder
}

func (r *LogCollector) Collect(span *Span) error {
	log.Println(string(r.encoder.Encode(span)))
	return nil
}

func (r *LogCollector) Close() error {
	return nil
}

func NewLogCollector() Collector {
	return &LogCollector{encoder: NewTraceEncoder(JSONEncoderType)}
}

type ConsoleCollector struct {
	encoder TraceEncoder
}

func (r *ConsoleCollector) Collect(span *Span) error {
	fmt.Println(string(r.encoder.Encode(span)))
	return nil
}

func (r *ConsoleCollector) Close() error {
	return nil
}

func NewConsoleCollector() Collector {
	return &ConsoleCollector{encoder: NewTraceEncoder(JSONEncoderType)}
}

// MultiCollector implements Collector by sending spans to all collectors.
type MultiCollector []Collector

func NewMultiCollector(collectors ...Collector) Collector {
	multiCollector := make([]Collector, len(collectors))
	for i := range collectors {
		multiCollector[i] = collectors[i]
	}

	return MultiCollector(multiCollector)
}

// Collect implements Collector.
func (c MultiCollector) Collect(s *Span) error {
	return c.aggregateErrors(func(coll Collector) error { return coll.Collect(s) })
}

// Close implements Collector.
func (c MultiCollector) Close() error {
	return c.aggregateErrors(func(coll Collector) error { return coll.Close() })
}

func (c MultiCollector) aggregateErrors(f func(Collector) error) error {
	for _, collector := range c {
		f(collector)
	}
	return nil
}