// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: kessel/inventory/v1beta2/authz/lookup.proto

package authz

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	KesselLookupService_LookupSubjects_FullMethodName  = "/kessel.inventory.v1beta2.authz.KesselLookupService/LookupSubjects"
	KesselLookupService_LookupResources_FullMethodName = "/kessel.inventory.v1beta2.authz.KesselLookupService/LookupResources"
)

// KesselLookupServiceClient is the client API for KesselLookupService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KesselLookupServiceClient interface {
	LookupSubjects(ctx context.Context, in *LookupSubjectsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LookupSubjectsResponse], error)
	LookupResources(ctx context.Context, in *LookupResourcesRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LookupResourcesResponse], error)
}

type kesselLookupServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKesselLookupServiceClient(cc grpc.ClientConnInterface) KesselLookupServiceClient {
	return &kesselLookupServiceClient{cc}
}

func (c *kesselLookupServiceClient) LookupSubjects(ctx context.Context, in *LookupSubjectsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LookupSubjectsResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &KesselLookupService_ServiceDesc.Streams[0], KesselLookupService_LookupSubjects_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[LookupSubjectsRequest, LookupSubjectsResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselLookupService_LookupSubjectsClient = grpc.ServerStreamingClient[LookupSubjectsResponse]

func (c *kesselLookupServiceClient) LookupResources(ctx context.Context, in *LookupResourcesRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[LookupResourcesResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &KesselLookupService_ServiceDesc.Streams[1], KesselLookupService_LookupResources_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[LookupResourcesRequest, LookupResourcesResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselLookupService_LookupResourcesClient = grpc.ServerStreamingClient[LookupResourcesResponse]

// KesselLookupServiceServer is the server API for KesselLookupService service.
// All implementations must embed UnimplementedKesselLookupServiceServer
// for forward compatibility.
type KesselLookupServiceServer interface {
	LookupSubjects(*LookupSubjectsRequest, grpc.ServerStreamingServer[LookupSubjectsResponse]) error
	LookupResources(*LookupResourcesRequest, grpc.ServerStreamingServer[LookupResourcesResponse]) error
	mustEmbedUnimplementedKesselLookupServiceServer()
}

// UnimplementedKesselLookupServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedKesselLookupServiceServer struct{}

func (UnimplementedKesselLookupServiceServer) LookupSubjects(*LookupSubjectsRequest, grpc.ServerStreamingServer[LookupSubjectsResponse]) error {
	return status.Errorf(codes.Unimplemented, "method LookupSubjects not implemented")
}
func (UnimplementedKesselLookupServiceServer) LookupResources(*LookupResourcesRequest, grpc.ServerStreamingServer[LookupResourcesResponse]) error {
	return status.Errorf(codes.Unimplemented, "method LookupResources not implemented")
}
func (UnimplementedKesselLookupServiceServer) mustEmbedUnimplementedKesselLookupServiceServer() {}
func (UnimplementedKesselLookupServiceServer) testEmbeddedByValue()                             {}

// UnsafeKesselLookupServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KesselLookupServiceServer will
// result in compilation errors.
type UnsafeKesselLookupServiceServer interface {
	mustEmbedUnimplementedKesselLookupServiceServer()
}

func RegisterKesselLookupServiceServer(s grpc.ServiceRegistrar, srv KesselLookupServiceServer) {
	// If the following call pancis, it indicates UnimplementedKesselLookupServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&KesselLookupService_ServiceDesc, srv)
}

func _KesselLookupService_LookupSubjects_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(LookupSubjectsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(KesselLookupServiceServer).LookupSubjects(m, &grpc.GenericServerStream[LookupSubjectsRequest, LookupSubjectsResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselLookupService_LookupSubjectsServer = grpc.ServerStreamingServer[LookupSubjectsResponse]

func _KesselLookupService_LookupResources_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(LookupResourcesRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(KesselLookupServiceServer).LookupResources(m, &grpc.GenericServerStream[LookupResourcesRequest, LookupResourcesResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselLookupService_LookupResourcesServer = grpc.ServerStreamingServer[LookupResourcesResponse]

// KesselLookupService_ServiceDesc is the grpc.ServiceDesc for KesselLookupService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KesselLookupService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "kessel.inventory.v1beta2.authz.KesselLookupService",
	HandlerType: (*KesselLookupServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "LookupSubjects",
			Handler:       _KesselLookupService_LookupSubjects_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "LookupResources",
			Handler:       _KesselLookupService_LookupResources_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "kessel/inventory/v1beta2/authz/lookup.proto",
}
