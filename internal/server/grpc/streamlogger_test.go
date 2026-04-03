package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type logEntry struct {
	level  log.Level
	keyval []interface{}
}

type mockLogger struct {
	entries []logEntry
}

func (m *mockLogger) Log(level log.Level, keyvals ...interface{}) error {
	m.entries = append(m.entries, logEntry{level: level, keyval: keyvals})
	return nil
}

func (m *mockLogger) findEntry(msgValue string) *logEntry {
	for i := range m.entries {
		for j := 0; j < len(m.entries[i].keyval); j += 2 {
			if m.entries[i].keyval[j] == "msg" && m.entries[i].keyval[j+1] == msgValue {
				return &m.entries[i]
			}
		}
	}
	return nil
}

func (m *mockLogger) getValue(entry *logEntry, key string) interface{} {
	for i := 0; i < len(entry.keyval); i += 2 {
		if entry.keyval[i] == key {
			return entry.keyval[i+1]
		}
	}
	return nil
}

type testMessage struct {
	value string
}

func (tm *testMessage) String() string {
	return fmt.Sprintf("msg:%s", tm.value)
}

type countingServerStream struct {
	grpc.ServerStream
	ctx       context.Context
	recvCount int
	recvErr   error
	sendCount int
	sendErr   error
	recvMsgs  []interface{}
	sendMsgs  []interface{}
}

func (c *countingServerStream) Context() context.Context {
	return c.ctx
}

func (c *countingServerStream) RecvMsg(m interface{}) error {
	if c.recvErr != nil {
		return c.recvErr
	}
	c.recvCount++
	c.recvMsgs = append(c.recvMsgs, m)
	if c.recvCount > 1 {
		return io.EOF
	}
	return nil
}

func (c *countingServerStream) SendMsg(m interface{}) error {
	if c.sendErr != nil {
		return c.sendErr
	}
	c.sendCount++
	c.sendMsgs = append(c.sendMsgs, m)
	return nil
}

func TestStreamLoggingInterceptor_LogsOpenAndClose(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)
		err = ss.SendMsg(&testMessage{value: "response"})
		require.NoError(t, err)
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	require.Len(t, mockLog.entries, 3, "Should log open, request args, and close")

	openEntry := mockLog.findEntry("stream opened")
	require.NotNil(t, openEntry)
	assert.Equal(t, log.LevelInfo, openEntry.level)
	assert.Equal(t, "server", mockLog.getValue(openEntry, "kind"))
	assert.Equal(t, "grpc", mockLog.getValue(openEntry, "component"))
	assert.Equal(t, "/test.Service/StreamMethod", mockLog.getValue(openEntry, "operation"))
	streamID := mockLog.getValue(openEntry, "stream.id")
	assert.NotEmpty(t, streamID)

	requestEntry := mockLog.findEntry("stream request")
	require.NotNil(t, requestEntry)
	assert.Equal(t, log.LevelInfo, requestEntry.level)
	assert.Equal(t, "/test.Service/StreamMethod", mockLog.getValue(requestEntry, "operation"))
	assert.Equal(t, streamID, mockLog.getValue(requestEntry, "stream.id"))
	assert.Equal(t, "msg:request", mockLog.getValue(requestEntry, "args"))

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, log.LevelInfo, closeEntry.level)
	assert.Equal(t, "server", mockLog.getValue(closeEntry, "kind"))
	assert.Equal(t, "grpc", mockLog.getValue(closeEntry, "component"))
	assert.Equal(t, "/test.Service/StreamMethod", mockLog.getValue(closeEntry, "operation"))
	assert.Equal(t, streamID, mockLog.getValue(closeEntry, "stream.id"), "stream.id should match between open and close")
	assert.Equal(t, int32(200), mockLog.getValue(closeEntry, "code"))
	assert.Equal(t, "", mockLog.getValue(closeEntry, "reason"))
	assert.Equal(t, "", mockLog.getValue(closeEntry, "stack"))
	assert.Equal(t, int64(1), mockLog.getValue(closeEntry, "sent"))
	assert.Equal(t, int64(1), mockLog.getValue(closeEntry, "received"))
	latency := mockLog.getValue(closeEntry, "latency")
	assert.IsType(t, float64(0), latency)
	assert.Greater(t, latency.(float64), 0.0)
}

func TestStreamLoggingInterceptor_LogsOpenOnlyOnFirstRecvMsg(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		for i := 0; i < 3; i++ {
			msg := &testMessage{value: fmt.Sprintf("request%d", i)}
			err := ss.RecvMsg(msg)
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	require.Len(t, mockLog.entries, 3, "Should log open, one request args, and close")
	openEntry := mockLog.findEntry("stream opened")
	require.NotNil(t, openEntry)
	requestEntries := 0
	for i := range mockLog.entries {
		for j := 0; j < len(mockLog.entries[i].keyval); j += 2 {
			if mockLog.entries[i].keyval[j] == "msg" && mockLog.entries[i].keyval[j+1] == "stream request" {
				requestEntries++
			}
		}
	}
	assert.Equal(t, 1, requestEntries, "Should log args only once even with multiple RecvMsg calls")
}

func TestStreamLoggingInterceptor_ErrorLogsAtErrorLevel(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	expectedErr := status.Error(codes.NotFound, "resource not found")
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)
		return expectedErr
	}

	err := interceptor(nil, stream, info, handler)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, log.LevelWarn, closeEntry.level, "NotFound should be logged at WARN level")
	assert.Equal(t, int32(404), mockLog.getValue(closeEntry, "code"))
	assert.Equal(t, "resource not found", mockLog.getValue(closeEntry, "reason"))
	stack := mockLog.getValue(closeEntry, "stack")
	assert.NotEmpty(t, stack)
	assert.Contains(t, stack.(string), "resource not found")
}

func TestStreamLoggingInterceptor_NonGRPCError(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	expectedErr := errors.New("generic error")
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)
		return expectedErr
	}

	err := interceptor(nil, stream, info, handler)
	require.Error(t, err)
	assert.Equal(t, expectedErr, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, log.LevelError, closeEntry.level)
	assert.Equal(t, int32(500), mockLog.getValue(closeEntry, "code"), "Unknown errors should map to 500")
	assert.Equal(t, "", mockLog.getValue(closeEntry, "reason"))
	stack := mockLog.getValue(closeEntry, "stack")
	assert.NotEmpty(t, stack)
}

func TestStreamLoggingInterceptor_CountsSentAndReceived(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)

		for i := 0; i < 5; i++ {
			err = ss.SendMsg(&testMessage{value: fmt.Sprintf("response%d", i)})
			require.NoError(t, err)
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, int64(5), mockLog.getValue(closeEntry, "sent"))
	assert.Equal(t, int64(1), mockLog.getValue(closeEntry, "received"))
}

func TestStreamLoggingInterceptor_SendMsgError_DoesNotIncrement(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	expectedErr := errors.New("send error")
	stream := &countingServerStream{
		ctx:     context.Background(),
		sendErr: expectedErr,
	}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)

		err = ss.SendMsg(&testMessage{value: "response"})
		return err
	}

	err := interceptor(nil, stream, info, handler)
	require.Error(t, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, int64(0), mockLog.getValue(closeEntry, "sent"), "Should not increment on SendMsg error")
	assert.Equal(t, int64(1), mockLog.getValue(closeEntry, "received"))
}

func TestStreamLoggingInterceptor_RecvMsgError_DoesNotIncrement(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	expectedErr := errors.New("recv error")
	stream := &countingServerStream{
		ctx:     context.Background(),
		recvErr: expectedErr,
	}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		return err
	}

	err := interceptor(nil, stream, info, handler)
	require.Error(t, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, int64(0), mockLog.getValue(closeEntry, "sent"))
	assert.Equal(t, int64(0), mockLog.getValue(closeEntry, "received"), "Should not increment on RecvMsg error")
}

func TestStreamLoggingInterceptor_NoRecvMsg_LogsOpenAndClose(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	require.Len(t, mockLog.entries, 2, "Should log both open and close even if no RecvMsg")
	openEntry := mockLog.findEntry("stream opened")
	require.NotNil(t, openEntry, "Should log open even if no successful RecvMsg")

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, int64(0), mockLog.getValue(closeEntry, "received"))
}

func TestExtractArgs_Stringer(t *testing.T) {
	msg := &testMessage{value: "test"}
	result := extractArgs(msg)
	assert.Equal(t, "msg:test", result)
}

func TestExtractArgs_NonStringer(t *testing.T) {
	type simpleStruct struct {
		Field string
	}
	msg := &simpleStruct{Field: "value"}
	result := extractArgs(msg)
	assert.Contains(t, result, "Field:value")
}

func TestExtractArgs_Nil(t *testing.T) {
	result := extractArgs(nil)
	assert.Equal(t, "<nil>", result)
}

func TestLoggingStream_ConcurrentSendRecv(t *testing.T) {
	mockLog := &mockLogger{}
	interceptor := newStreamLoggingInterceptor(mockLog)

	stream := &countingServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		msg := &testMessage{value: "request"}
		err := ss.RecvMsg(msg)
		require.NoError(t, err)

		done := make(chan struct{})
		errCh := make(chan error, 2)

		go func() {
			for i := 0; i < 10; i++ {
				if err := ss.SendMsg(&testMessage{value: fmt.Sprintf("response%d", i)}); err != nil {
					errCh <- err
					return
				}
			}
			close(done)
		}()

		<-done
		close(errCh)
		for err := range errCh {
			return err
		}
		return nil
	}

	err := interceptor(nil, stream, info, handler)
	require.NoError(t, err)

	closeEntry := mockLog.findEntry("stream closed")
	require.NotNil(t, closeEntry)
	assert.Equal(t, int64(10), mockLog.getValue(closeEntry, "sent"))
	assert.Equal(t, int64(1), mockLog.getValue(closeEntry, "received"))
}

func TestStreamLoggingInterceptor_DifferentGRPCCodes(t *testing.T) {
	testCases := []struct {
		name          string
		grpcCode      codes.Code
		expectedHTTP  int32
		expectedLevel log.Level
		message       string
	}{
		{"OK", codes.OK, 200, log.LevelInfo, ""},
		{"Canceled", codes.Canceled, 499, log.LevelInfo, "request canceled"},
		{"InvalidArgument", codes.InvalidArgument, 400, log.LevelWarn, "invalid argument"},
		{"NotFound", codes.NotFound, 404, log.LevelWarn, "not found"},
		{"AlreadyExists", codes.AlreadyExists, 409, log.LevelWarn, "already exists"},
		{"PermissionDenied", codes.PermissionDenied, 403, log.LevelWarn, "permission denied"},
		{"Unauthenticated", codes.Unauthenticated, 401, log.LevelWarn, "unauthenticated"},
		{"ResourceExhausted", codes.ResourceExhausted, 429, log.LevelWarn, "resource exhausted"},
		{"FailedPrecondition", codes.FailedPrecondition, 400, log.LevelWarn, "failed precondition"},
		{"Aborted", codes.Aborted, 409, log.LevelWarn, "aborted"},
		{"OutOfRange", codes.OutOfRange, 400, log.LevelWarn, "out of range"},
		{"Unimplemented", codes.Unimplemented, 501, log.LevelError, "unimplemented"},
		{"Internal", codes.Internal, 500, log.LevelError, "internal error"},
		{"Unavailable", codes.Unavailable, 503, log.LevelError, "unavailable"},
		{"DataLoss", codes.DataLoss, 500, log.LevelError, "data loss"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockLog := &mockLogger{}
			interceptor := newStreamLoggingInterceptor(mockLog)

			stream := &countingServerStream{ctx: context.Background()}
			info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}

			var expectedErr error
			if tc.grpcCode != codes.OK {
				expectedErr = status.Error(tc.grpcCode, tc.message)
			}

			handler := func(srv interface{}, ss grpc.ServerStream) error {
				if tc.grpcCode != codes.OK {
					msg := &testMessage{value: "request"}
					err := ss.RecvMsg(msg)
					require.NoError(t, err)
				}
				return expectedErr
			}

			err := interceptor(nil, stream, info, handler)
			if tc.grpcCode != codes.OK {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			closeEntry := mockLog.findEntry("stream closed")
			require.NotNil(t, closeEntry)
			assert.Equal(t, tc.expectedHTTP, mockLog.getValue(closeEntry, "code"))
			assert.Equal(t, tc.message, mockLog.getValue(closeEntry, "reason"))
			assert.Equal(t, tc.expectedLevel, closeEntry.level)
		})
	}
}
