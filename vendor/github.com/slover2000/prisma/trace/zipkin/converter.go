package zipkin

import (
	"time"
	"errors"
	"encoding/hex"
	"encoding/binary"

	"github.com/slover2000/prisma/utils"
	"github.com/slover2000/prisma/trace"	
	"github.com/slover2000/prisma/trace/zipkin/thrift-gen/zipkincore"
)

// ConvertToZipkinSpan converts a span to zipkin format span
func ConvertToZipkinSpan(s *trace.Span) (*zipkincore.Span, error) {
	tid, err := hex.DecodeString(s.TraceID())
	if err != nil {
		return nil, err
	}

	low := int64(binary.BigEndian.Uint64(tid[8:]))
	high := int64(binary.BigEndian.Uint64(tid[:8]))	
	var parentID *int64
	if s.ParentSpanID() > 0 {
		parent := int64(s.ParentSpanID())
		parentID = &parent
	}
	start := s.StartTime(time.Microsecond)
	end := s.EndTime(time.Microsecond)
	duration := end - start
	if duration <= 0 {
		duration = 1
	}

	zipkinSpan := &zipkincore.Span{
		TraceID: low,
		TraceIDHigh: &high,
		Name: s.Name(),
		ID: int64(s.SpanID()),
		ParentID: parentID,
		Timestamp: &start,
		Duration: &duration,
		Annotations: make([]*zipkincore.Annotation, 0),
		BinaryAnnotations: make([]*zipkincore.BinaryAnnotation, 0),
	}

	ips, err := utils.LocalIPs()
	if err != nil || len(ips) == 0 {
		return nil, errors.New("can't get host ip address")
	}
	
	var ip int32
	for _, v := range ips {
		if ip, err = utils.IPToInt32(v); err == nil {
			break
		}
	}
	ep := &zipkincore.Endpoint{
		Ipv4: ip,
		ServiceName: s.ServiceName(),
	}

	kind := s.Kind()
	if kind == trace.SpanKindClient {
		sendAnn := &zipkincore.Annotation{
			Timestamp: start,
			Value: zipkincore.CLIENT_SEND,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, sendAnn)

		recvAnn := &zipkincore.Annotation{
			Timestamp: end,
			Value: zipkincore.CLIENT_RECV,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, recvAnn)
	} else if kind == trace.SpanKindServer {
		recvAnn := &zipkincore.Annotation{
			Timestamp: start,
			Value: zipkincore.SERVER_RECV,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, recvAnn)

		sendAnn := &zipkincore.Annotation{
			Timestamp: end,
			Value: zipkincore.SERVER_SEND,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, sendAnn)
	} else {
		sendAnn := &zipkincore.Annotation{
			Timestamp: start,
			Value: zipkincore.WIRE_SEND,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, sendAnn)

		recvAnn := &zipkincore.Annotation{
			Timestamp: end,
			Value: zipkincore.WIRE_RECV,
			Host: ep,
		}
		zipkinSpan.Annotations = append(zipkinSpan.Annotations, recvAnn)
	}

	labels := s.Labels()
	for k, v := range labels {		
		target := k
		switch k {
		case trace.LabelHTTPHost:
			target = zipkincore.HTTP_HOST
		case trace.LabelHTTPMethod:
			target = zipkincore.HTTP_METHOD
		case trace.LabelHTTPURL:
			target = zipkincore.HTTP_URL
		case trace.LabelHTTPStatusCode:
			target = zipkincore.HTTP_STATUS_CODE
		case trace.LabelError:
			target = zipkincore.ERROR
		}
		binAnn := &zipkincore.BinaryAnnotation{
			Key: target,
			Value: []byte(v),
			AnnotationType: zipkincore.AnnotationType_STRING,
		}
		zipkinSpan.BinaryAnnotations = append(zipkinSpan.BinaryAnnotations, binAnn)			
	}

	return zipkinSpan, nil
}