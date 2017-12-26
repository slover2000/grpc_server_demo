package prisma

import (
	"time"
	"fmt"
	"io"
	"sync"
	"encoding/hex"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/slover2000/prisma/trace"
)

const grpcMetadataKey = "x-trace-bin"

// GRPCUnaryClientInterceptor returns a grpc.UnaryClientInterceptor that traces all outgoing requests from a gRPC client.
// The calling context should already have a *trace.Span; a child span will be
// created for the outgoing gRPC call. If the calling context doesn't have a span,
// the call will not be traced. If the client is nil, then the interceptor just
// passes through the request.
//
// The functionality in gRPC that this feature relies on is currently experimental.
func (c *InterceptorClient) GRPCUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	if c == nil {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}
	return grpc.UnaryClientInterceptor(c.grpcUnaryInterceptor)
}

func (c *InterceptorClient) grpcUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	span := trace.FromContext(ctx).NewChild(method)
	if span == nil {
		span = c.trace.NewClientKindSpanOrNot(method)
	}
	defer span.Finish()
	
	outgoingCtx := buildClientOutgoingContext(ctx, span)

	startTime := time.Now()
	err := invoker(outgoingCtx, method, req, reply, cc, opts...)
	if err != nil {
		span.SetLabel(trace.LabelError, err.Error())
	}

	// do metrics
	c.grpcClientMetrics.CounterGRPC(method, time.Since(startTime), err)

	// log unary client grpc	
	msg := fmt.Sprintf("unary %s %s", method, grpc.Code(err).String())
	c.log.LogGrpcClientLine(outgoingCtx, method, startTime, err, msg)
	return err
}

// GRPCStreamClientInterceptor returns a grpc.StreamClientInterceptor that traces all outgoing requests from a gRPC client.
func (c *InterceptorClient) GRPCStreamClientInterceptor() grpc.StreamClientInterceptor {
	if c == nil {
		return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(ctx, desc, cc, method, opts...)
		}
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		span := trace.FromContext(ctx).NewChild(method)
		if span == nil {
			span = c.trace.NewClientKindSpanOrNot(method)
		}

		outgoingCtx := buildClientOutgoingContext(ctx, span)
		clientStream, err := streamer(outgoingCtx, desc, cc, method, opts...)
		if err != nil {		
			finishClientSpan(outgoingCtx, c, method, span, time.Now(), err)
			return nil, err
		}
		return &tracedClientStream{ClientStream: clientStream, method: method, startTime: time.Now(), client: c, clientSpan: span}, nil
	}
}

func buildClientOutgoingContext(parentCtx context.Context, span *trace.Span) context.Context {
	if span == nil {
		return parentCtx
	}

	// traceID is a hex-encoded 128-bit value.
	// TODO(jbd): Decode trace IDs upon arrival and
	// represent trace IDs with 16 bytes internally.
	tid, err := hex.DecodeString(span.TraceID())
	if err != nil {
		return parentCtx		
	}

	traceContext := make([]byte, trace.TraceContextLen)
	trace.PackTrace(traceContext, tid, span.SpanID(), byte(span.TraceGloablOptions()))
	md, ok := metadata.FromOutgoingContext(parentCtx)
	if !ok {
		md = metadata.Pairs(grpcMetadataKey, string(traceContext))
	} else {
		md = md.Copy() // metadata is immutable, copy.
		md[grpcMetadataKey] = []string{string(traceContext)}
	}
	ctx := metadata.NewOutgoingContext(parentCtx, md)
	
	return ctx
}

// type serverStreamingRetryingStream is the implementation of grpc.ClientStream that acts as a
// proxy to the underlying call. If any of the RecvMsg() calls fail, it will try to reestablish
// a new ClientStream according to the retry policy.
type tracedClientStream struct {
	grpc.ClientStream
	mu              	sync.Mutex
	alreadyFinished 	bool
	method              string
	startTime			time.Time
	client 				*InterceptorClient
	clientSpan			*trace.Span
}

func (s *tracedClientStream) Header() (metadata.MD, error) {
	h, err := s.ClientStream.Header()
	if err != nil {
		s.finishClientSpan(err)
	}
	return h, err
}

func (s *tracedClientStream) SendMsg(m interface{}) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.finishClientSpan(err)
	}
	return err
}

func (s *tracedClientStream) RecvMsg(m interface{}) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		s.finishClientSpan(err)
	}
	return err
}

func (s *tracedClientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	s.finishClientSpan(err)
	return err
}

func (s *tracedClientStream) finishClientSpan(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.alreadyFinished {
		finishClientSpan(s.Context(), s.client, s.method, s.clientSpan, s.startTime, err)
		s.alreadyFinished = true
	}
}

func finishClientSpan(ctx context.Context, client *InterceptorClient, method string, clientSpan *trace.Span, startTime time.Time, err error) {		
	var logErr error
	if err != nil && err != io.EOF {
		logErr = err
	}

	// do metrics
	client.grpcClientMetrics.CounterGRPC(method, time.Since(startTime), logErr)

	// log stream client grpc
	msg := fmt.Sprintf("stream %s %s", method, grpc.Code(logErr).String())
	client.log.LogGrpcClientLine(ctx, method, startTime, logErr, msg)

	if err != nil && err != io.EOF {
		clientSpan.SetLabel(trace.LabelError, err.Error())
	}
	clientSpan.Finish()
}

// GRPCUnaryServerInterceptor returns a grpc.UnaryServerInterceptor that enables the tracing of the incoming
// gRPC calls. Incoming call's context can be used to extract the span on servers that enabled this option:
//
//	span := trace.FromContext(ctx)
//
// If the client is nil, then the interceptor just invokes the handler.
//
// The functionality in gRPC that this feature relies on is currently experimental.
func (c *InterceptorClient) GRPCUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	if c == nil {
		return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, _ := metadata.FromIncomingContext(ctx)
		var span *trace.Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.trace.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}
		
		ctx = trace.NewContext(ctx, span)
		startTime := time.Now()
		resp, err = handler(ctx, req)

		// do metrics
		c.grpcServerMetrics.CounterGRPC(info.FullMethod, time.Since(startTime), err)

		// log unary server grpc
		msg := fmt.Sprintf("unary %s %s", info.FullMethod, grpc.Code(err).String())
		c.log.LogGrpcServerLine(ctx, info.FullMethod, startTime, err, msg)
		return
	}
}

// WrappedServerStream is a thin wrapper around grpc.ServerStream that allows modifying context.
type WrappedServerStream struct {
	grpc.ServerStream
	// WrappedContext is the wrapper's own Context. You can assign it.
	WrappedContext context.Context
}

// Context returns the wrapper's WrappedContext, overwriting the nested grpc.ServerStream.Context()
func (w *WrappedServerStream) Context() context.Context {
	return w.WrappedContext
}

// WrapServerStream returns a ServerStream that has the ability to overwrite context.
func wrapServerStream(stream grpc.ServerStream) *WrappedServerStream {
	if existing, ok := stream.(*WrappedServerStream); ok {
		return existing
	}
	return &WrappedServerStream{ServerStream: stream, WrappedContext: stream.Context()}
}

// GRPStreamServerInterceptor returns a new streaming server interceptor for OpenTracing.
func (c *InterceptorClient) GRPStreamServerInterceptor() grpc.StreamServerInterceptor {
	if c == nil {
		return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, stream)
		}
	}

	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		md, _ := metadata.FromIncomingContext(ctx)
		var span *trace.Span
		if header, ok := md[grpcMetadataKey]; ok {
			span = c.trace.SpanFromContext(info.FullMethod, header[0])
			defer span.Finish()
		}
		
		ctx = trace.NewContext(ctx, span)
		wrappedStream := wrapServerStream(stream)
		wrappedStream.WrappedContext = ctx
		startTime := time.Now()
		err := handler(srv, wrappedStream)

		// do metrics
		c.grpcServerMetrics.CounterGRPC(info.FullMethod, time.Since(startTime), err)

		// log stream server grpc
		msg := fmt.Sprintf("stream %s %s", info.FullMethod, grpc.Code(err).String())
		c.log.LogGrpcServerLine(ctx, info.FullMethod, startTime, err, msg)
		return err
	}
}