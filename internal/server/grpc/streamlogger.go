package grpc

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	httpstatus "github.com/go-kratos/kratos/v2/transport/http/status"
)

func newStreamLoggingInterceptor(logger log.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		startTime := time.Now()
		streamID := uuid.New().String()
		helper := log.NewHelper(logger)

		wrapped := &loggingStream{
			ServerStream: ss,
			logger:       helper,
			operation:    info.FullMethod,
			streamID:     streamID,
		}

		err := handler(srv, wrapped)

		code := codes.OK
		reason := ""
		stack := ""
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
				reason = st.Message()
			} else {
				code = codes.Unknown
			}
			stack = fmt.Sprintf("%+v", err)
		}

		level := log.LevelInfo
		if err != nil {
			level = log.LevelError
		}

		helper.Log(level,
			"kind", "server",
			"component", "grpc",
			"operation", info.FullMethod,
			"stream.id", streamID,
			"msg", "stream closed",
			"code", int32(httpstatus.FromGRPCCode(code)),
			"reason", reason,
			"stack", stack,
			"sent", wrapped.sent.Load(),
			"received", wrapped.received.Load(),
			"latency", time.Since(startTime).Seconds(),
		)

		return err
	}
}

func extractArgs(msg interface{}) string {
	if stringer, ok := msg.(fmt.Stringer); ok {
		return stringer.String()
	}
	return fmt.Sprintf("%+v", msg)
}

type loggingStream struct {
	grpc.ServerStream
	logger      *log.Helper
	operation   string
	streamID    string
	sent        atomic.Int64
	received    atomic.Int64
	logOpenOnce sync.Once
}

func (s *loggingStream) SendMsg(m interface{}) error {
	err := s.ServerStream.SendMsg(m)
	if err == nil {
		s.sent.Add(1)
	}
	return err
}

func (s *loggingStream) RecvMsg(m interface{}) error {
	err := s.ServerStream.RecvMsg(m)
	if err == nil {
		s.logOpenOnce.Do(func() {
			s.logger.Infow(
				"kind", "server",
				"component", "grpc",
				"operation", s.operation,
				"stream.id", s.streamID,
				"msg", "stream opened",
				"args", extractArgs(m),
			)
		})
		s.received.Add(1)
	}
	return err
}
