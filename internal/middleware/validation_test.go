package middleware

import (
	"context"
	"testing"

	"buf.build/go/protovalidate"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type dummyServerStream struct {
	grpc.ServerStream
	recvMsgFunc func(msg interface{}) error
}

func (d *dummyServerStream) RecvMsg(msg interface{}) error {
	return d.recvMsgFunc(msg)
}

func (d *dummyServerStream) Context() context.Context {
	return context.Background()
}

func (d *dummyServerStream) SetHeader(metadata.MD) error  { return nil }
func (d *dummyServerStream) SendHeader(metadata.MD) error { return nil }
func (d *dummyServerStream) SetTrailer(metadata.MD)       {}
func (d *dummyServerStream) SendMsg(interface{}) error    { return nil }

func TestStreamValidationInterceptor_ValidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	interceptor := StreamValidationInterceptor(validator)

	dummyStream := &dummyServerStream{
		recvMsgFunc: func(msg interface{}) error {
			*msg.(*pb.StreamedListObjectsRequest) = pb.StreamedListObjectsRequest{
				ObjectType: &pb.RepresentationType{ResourceType: "host"},
				Relation:   "viewer",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "user",
						ResourceId:   "alice",
					},
				},
			}
			return nil
		},
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		msg := &pb.StreamedListObjectsRequest{}
		return stream.RecvMsg(msg)
	}

	err = interceptor(nil, dummyStream, nil, handler)
	assert.NoError(t, err)
}

func TestStreamValidationInterceptor_InvalidRequest(t *testing.T) {
	t.Parallel()

	validator, err := protovalidate.New()
	assert.NoError(t, err)

	interceptor := StreamValidationInterceptor(validator)

	dummyStream := &dummyServerStream{
		recvMsgFunc: func(msg interface{}) error {
			*msg.(*pb.StreamedListObjectsRequest) = pb.StreamedListObjectsRequest{}
			return nil
		},
	}

	handler := func(srv interface{}, stream grpc.ServerStream) error {
		msg := &pb.StreamedListObjectsRequest{}
		return stream.RecvMsg(msg)
	}

	err = interceptor(nil, dummyStream, nil, handler)
	assert.Error(t, err)
}
