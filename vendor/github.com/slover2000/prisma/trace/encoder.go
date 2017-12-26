package trace

import (
	"time"
	"encoding/binary"
	"encoding/json"
)

const (
	versionID    = 0
	traceIDField = 0
	spanIDField  = 1
	optsField    = 2

	traceIDLen = 16
	spanIDLen  = 8
	optsLen    = 1

	// Len represents the length of trace context.
	TraceContextLen = 1 + 1 + traceIDLen + 1 + spanIDLen + 1 + optsLen
)

// PackTrace encodes trace ID, span ID and options into dst. The number of bytes
// written will be returned. If len(dst) isn't big enough to fit the trace context,
// a negative number is returned.
func PackTrace(dst []byte, traceID []byte, spanID uint64, opts byte) (n int) {
	if len(dst) < TraceContextLen {
		return -1
	}
	var offset = 0
	putByte := func(b byte) { dst[offset] = b; offset++ }
	putUint64 := func(u uint64) { binary.LittleEndian.PutUint64(dst[offset:], u); offset += 8 }

	putByte(versionID)
	putByte(traceIDField)
	for _, b := range traceID {
		putByte(b)
	}
	putByte(spanIDField)
	putUint64(spanID)
	putByte(optsField)
	putByte(opts)

	return offset
}

// UnpackTrace decodes the src into a trace ID, span ID and options. If src doesn't
// contain a valid trace context, ok = false is returned.
func UnpackTrace(src []byte) (traceID []byte, spanID uint64, opts byte, ok bool) {
	if len(src) < TraceContextLen {
		return traceID, spanID, 0, false
	}
	var offset = 0
	readByte := func() byte { b := src[offset]; offset++; return b }
	readUint64 := func() uint64 { v := binary.LittleEndian.Uint64(src[offset:]); offset += 8; return v }

	if readByte() != versionID {
		return traceID, spanID, 0, false
	}
	for offset < len(src) {
		switch readByte() {
		case traceIDField:
			traceID = src[offset : offset+traceIDLen]
			offset += traceIDLen
		case spanIDField:
			spanID = readUint64()
		case optsField:
			opts = readByte()
		}
	}
	return traceID, spanID, opts, true
}

// TraceEncoder encode span data
type TraceEncoder interface {
	Encode(*Span) []byte
}

type TraceEncoderType int

const (
	// JSONEncoderType type of trace encoders
	JSONEncoderType TraceEncoderType = iota
)

func NewTraceEncoder(t TraceEncoderType) TraceEncoder {
	switch t {
	case  JSONEncoderType:
		return &JSONEncoder{}
	default:
		return nil
	}
}

type (
// JSONEncoder encoder for json format
 	JSONEncoder struct {
	
	}
	
	JSONTraceData struct {
		ServiceName	string 	`json:"service"`
		TraceID 	string	`json:"trace"`
		Span		JSONSpanData `json:"span"`
	}
	
	JSONSpanData struct {
		StartTime	int64	`json:"starttime"`
		EndTime 	int64	`json:"endtime"`
		Name 		string 	`json:"name"`
		Kind 		string 	`json:"kind"`
		SpanID 		uint64	`json:"spanid"`
		ParentID	uint64	`json:"parentid"`
		Labels		map[string]string	`json:"labels"`
	}
)

func (r *JSONEncoder) Encode(s *Span) []byte {
	data := JSONTraceData{
		ServiceName: s.trace.client.serviceName,
		TraceID: s.TraceID(),
		Span: JSONSpanData{
			StartTime: s.StartTime(time.Microsecond),
			EndTime: s.EndTime(time.Microsecond),
			Name: s.name,
			Kind: s.kind,
			SpanID: s.spanID,
			ParentID: s.parentSpanID,
			Labels: s.labels,
		},
	}
	result, _ := json.Marshal(data)
	return result
}