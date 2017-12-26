package logging

import (
	"fmt"
	"time"
	"path"
	"strconv"
	"net/http"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const (
	SystemField 		= "system"
	KindField 			= "span.kind"
	SpanKindClient      = `client`
	SpanKindServer      = `server`
	RequestTimeField    = "request.time"
	MethodField         = "method"
	DurationField       = "duration"
	CodeField           = "code"
	
	GRPCServiceField	= "service"		

	HTTPHostField       = `host`	
	HTTPRequestSizeField= `request.size`	
	HTTPURLField        = `url`
	HTTPUserAgentField  = `user_agent`	
)

type LogLevel = int
const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

type loggingOptions struct {
	level			LogLevel
	entry 			*logrus.Entry
}

func grpcErrorToCode(err error) codes.Code {
	return grpc.Code(err)
}

// Client is a client for logging
// A nil Client will no-op for all of its methods.
type Client struct {
	options	loggingOptions
}

// NewClient create a new client fo logging
func NewClient(level LogLevel, entry *logrus.Entry) (*Client, error) {
	client := &Client{
		options: loggingOptions{
			level: level,
			entry: entry,
		},
	}

	return client, nil
}

func converToMillisecond(duration time.Duration) int64 {
	ms := duration.Nanoseconds() / int64(time.Millisecond)
	if ms == 0 {
		ms = 1
	}
	return ms
}

func (c *Client) LogGrpcClientLine(ctx context.Context, fullMethodString string, startTime time.Time, err error, msg string) {
	if c == nil {
		return
	}

	code := grpc.Code(err)
	level := grpcCodeToLogrusLevel(code)
	if loglevelToLogusLevel(c.options.level) >= level {
		durVal := time.Since(startTime)
		fields := newGrpcClientLoggerFields(ctx, fullMethodString)
		fields[RequestTimeField] = startTime.Format("2006-01-02 15:04:05")
		fields[CodeField] = code.String()
		fields[DurationField] = fmt.Sprintf("%d", converToMillisecond(durVal))
		if err != nil {
			fields[logrus.ErrorKey] = err
		}
	
		logMessageWithLevel(c.options.entry.WithFields(fields), level, msg)	
	}
}

func (c *Client) LogGrpcServerLine(ctx context.Context, fullMethodString string, startTime time.Time, err error, msg string) {
	if c == nil {
		return
	}

	code := grpc.Code(err)
	level := grpcCodeToLogrusLevel(code)
	if loglevelToLogusLevel(c.options.level) >= level {
		durVal := time.Since(startTime)
		fields := newGrpcServerLoggerFields(ctx, fullMethodString)
		fields[RequestTimeField] = startTime.Format("2006-01-02 15:04:05")
		fields[CodeField] = code.String()
		fields[DurationField] = fmt.Sprintf("%d", converToMillisecond(durVal))
		if err != nil {
			fields[logrus.ErrorKey] = err
		}
	
		logMessageWithLevel(c.options.entry.WithFields(fields), level, msg)	
	}
}

func (c *Client)LogHttpClientLine(req *http.Request, startTime time.Time, code int, msg string) {
	if c == nil {
		return
	}

	level := httpCodeToLogrusLevel(code)
	if loglevelToLogusLevel(c.options.level) >= level {
		durVal := time.Since(startTime)
		fields := newHttpClientLoggerFields(req)
		fields[RequestTimeField] = startTime.Format("2006-01-02 15:04:05")		
		fields[CodeField] = strconv.Itoa(code)
		fields[DurationField] = fmt.Sprintf("%d", converToMillisecond(durVal))
		logMessageWithLevel(c.options.entry.WithFields(fields), level, msg)
	}
}
func (c *Client)LogHttpServerLine(req *http.Request, startTime time.Time, code int, msg string) {
	if c == nil {
		return
	}

	level := httpCodeToLogrusLevel(code)
	if loglevelToLogusLevel(c.options.level) >= level {
		durVal := time.Since(startTime)
		fields := newHttpServerLoggerFields(req)
		fields[RequestTimeField] = startTime.Format("2006-01-02 15:04:05")
		fields[CodeField] = strconv.Itoa(code)
		fields[DurationField] = fmt.Sprintf("%d", converToMillisecond(durVal))
		logMessageWithLevel(c.options.entry.WithFields(fields), level, msg)
	}
}

func newGrpcClientLoggerFields(ctx context.Context, fullMethodString string) logrus.Fields {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	return logrus.Fields{
		SystemField:     "grpc",
		KindField:       SpanKindClient,
		GRPCServiceField:service,
		MethodField:     method,
	}
}

func newGrpcServerLoggerFields(ctx context.Context, fullMethodString string) logrus.Fields {
	service := path.Dir(fullMethodString)[1:]
	method := path.Base(fullMethodString)
	return logrus.Fields{
		SystemField:     "grpc",
		KindField:       SpanKindServer,
		GRPCServiceField:service,
		MethodField:     method,
	}
}

func newHttpClientLoggerFields(req *http.Request) logrus.Fields {
	url := req.URL.String()
	method := req.Method
	return logrus.Fields{
		SystemField:    "http",
		KindField:      SpanKindClient,
		HTTPURLField:   url,
		MethodField:    method,
	}
}

func newHttpServerLoggerFields(req *http.Request) logrus.Fields {
	url := req.URL.String()
	method := req.Method
	if req.ContentLength > 0 {
		return logrus.Fields{
			SystemField:   	"http",
			KindField:      SpanKindServer,
			HTTPURLField:   url,
			MethodField:    method,
			HTTPUserAgentField:  req.UserAgent(),
			HTTPRequestSizeField:req.ContentLength,
		}
	} else {
		return logrus.Fields{
			SystemField:   	"http",
			KindField:      SpanKindServer,
			HTTPURLField:   url,
			MethodField:    method,
			HTTPUserAgentField:  req.UserAgent(),			
		}		
	}
}

func logMessageWithLevel(entry *logrus.Entry, level logrus.Level, msg string) {
	switch level {
	case logrus.DebugLevel:
		entry.Debug(msg)
	case logrus.InfoLevel:
		entry.Info(msg)
	case logrus.WarnLevel:
		entry.Warn(msg)
	case logrus.ErrorLevel:
		entry.Error(msg)
	case logrus.FatalLevel:
		entry.Fatal(msg)
	}
}

func loglevelToLogusLevel(lvl LogLevel) logrus.Level {
	switch lvl {
	case DebugLevel:
		return logrus.DebugLevel
	case InfoLevel:
		return logrus.InfoLevel
	case WarnLevel:
		return logrus.WarnLevel
	case ErrorLevel:
		return logrus.ErrorLevel
	case FatalLevel:
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

// grpcCodeToLogrusLevel is the default implementation of gRPC return codes to log levels for server side.
func grpcCodeToLogrusLevel(code codes.Code) logrus.Level {
	switch code {
	case codes.OK:
		return logrus.InfoLevel
	case codes.Canceled:
		return logrus.InfoLevel
	case codes.Unknown:
		return logrus.ErrorLevel
	case codes.InvalidArgument:
		return logrus.InfoLevel
	case codes.DeadlineExceeded:
		return logrus.WarnLevel
	case codes.NotFound:
		return logrus.InfoLevel
	case codes.AlreadyExists:
		return logrus.InfoLevel
	case codes.PermissionDenied:
		return logrus.WarnLevel
	case codes.Unauthenticated:
		return logrus.InfoLevel // unauthenticated requests can happen
	case codes.ResourceExhausted:
		return logrus.WarnLevel
	case codes.FailedPrecondition:
		return logrus.WarnLevel
	case codes.Aborted:
		return logrus.WarnLevel
	case codes.OutOfRange:
		return logrus.WarnLevel
	case codes.Unimplemented:
		return logrus.ErrorLevel
	case codes.Internal:
		return logrus.ErrorLevel
	case codes.Unavailable:
		return logrus.WarnLevel
	case codes.DataLoss:
		return logrus.ErrorLevel
	default:
		return logrus.ErrorLevel
	}
}

func httpCodeToLogrusLevel(status int) logrus.Level {
	if status > 200 {
		return logrus.ErrorLevel
	} else if status == 0 || (status >= 200 && status <= 299) {
		return logrus.InfoLevel
	} else {
		return logrus.WarnLevel
	}
}