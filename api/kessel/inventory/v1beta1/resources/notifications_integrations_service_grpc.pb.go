// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: kessel/inventory/v1beta1/resources/notifications_integrations_service.proto

package resources

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
	KesselNotificationsIntegrationService_CreateNotificationsIntegration_FullMethodName  = "/kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService/CreateNotificationsIntegration"
	KesselNotificationsIntegrationService_UpdateNotificationsIntegration_FullMethodName  = "/kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService/UpdateNotificationsIntegration"
	KesselNotificationsIntegrationService_UpdateNotificationsIntegrations_FullMethodName = "/kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService/UpdateNotificationsIntegrations"
	KesselNotificationsIntegrationService_DeleteNotificationsIntegration_FullMethodName  = "/kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService/DeleteNotificationsIntegration"
	KesselNotificationsIntegrationService_ListNotificationsIntegrations_FullMethodName   = "/kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService/ListNotificationsIntegrations"
)

// KesselNotificationsIntegrationServiceClient is the client API for KesselNotificationsIntegrationService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KesselNotificationsIntegrationServiceClient interface {
	CreateNotificationsIntegration(ctx context.Context, in *CreateNotificationsIntegrationRequest, opts ...grpc.CallOption) (*CreateNotificationsIntegrationResponse, error)
	UpdateNotificationsIntegration(ctx context.Context, in *UpdateNotificationsIntegrationRequest, opts ...grpc.CallOption) (*UpdateNotificationsIntegrationResponse, error)
	// deprecated Temporary streaming update endpoint
	UpdateNotificationsIntegrations(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse], error)
	DeleteNotificationsIntegration(ctx context.Context, in *DeleteNotificationsIntegrationRequest, opts ...grpc.CallOption) (*DeleteNotificationsIntegrationResponse, error)
	ListNotificationsIntegrations(ctx context.Context, in *ListNotificationsIntegrationsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ListNotificationsIntegrationsResponse], error)
}

type kesselNotificationsIntegrationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKesselNotificationsIntegrationServiceClient(cc grpc.ClientConnInterface) KesselNotificationsIntegrationServiceClient {
	return &kesselNotificationsIntegrationServiceClient{cc}
}

func (c *kesselNotificationsIntegrationServiceClient) CreateNotificationsIntegration(ctx context.Context, in *CreateNotificationsIntegrationRequest, opts ...grpc.CallOption) (*CreateNotificationsIntegrationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateNotificationsIntegrationResponse)
	err := c.cc.Invoke(ctx, KesselNotificationsIntegrationService_CreateNotificationsIntegration_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kesselNotificationsIntegrationServiceClient) UpdateNotificationsIntegration(ctx context.Context, in *UpdateNotificationsIntegrationRequest, opts ...grpc.CallOption) (*UpdateNotificationsIntegrationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UpdateNotificationsIntegrationResponse)
	err := c.cc.Invoke(ctx, KesselNotificationsIntegrationService_UpdateNotificationsIntegration_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kesselNotificationsIntegrationServiceClient) UpdateNotificationsIntegrations(ctx context.Context, opts ...grpc.CallOption) (grpc.ClientStreamingClient[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &KesselNotificationsIntegrationService_ServiceDesc.Streams[0], KesselNotificationsIntegrationService_UpdateNotificationsIntegrations_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselNotificationsIntegrationService_UpdateNotificationsIntegrationsClient = grpc.ClientStreamingClient[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]

func (c *kesselNotificationsIntegrationServiceClient) DeleteNotificationsIntegration(ctx context.Context, in *DeleteNotificationsIntegrationRequest, opts ...grpc.CallOption) (*DeleteNotificationsIntegrationResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteNotificationsIntegrationResponse)
	err := c.cc.Invoke(ctx, KesselNotificationsIntegrationService_DeleteNotificationsIntegration_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kesselNotificationsIntegrationServiceClient) ListNotificationsIntegrations(ctx context.Context, in *ListNotificationsIntegrationsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ListNotificationsIntegrationsResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &KesselNotificationsIntegrationService_ServiceDesc.Streams[1], KesselNotificationsIntegrationService_ListNotificationsIntegrations_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[ListNotificationsIntegrationsRequest, ListNotificationsIntegrationsResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselNotificationsIntegrationService_ListNotificationsIntegrationsClient = grpc.ServerStreamingClient[ListNotificationsIntegrationsResponse]

// KesselNotificationsIntegrationServiceServer is the server API for KesselNotificationsIntegrationService service.
// All implementations must embed UnimplementedKesselNotificationsIntegrationServiceServer
// for forward compatibility.
type KesselNotificationsIntegrationServiceServer interface {
	CreateNotificationsIntegration(context.Context, *CreateNotificationsIntegrationRequest) (*CreateNotificationsIntegrationResponse, error)
	UpdateNotificationsIntegration(context.Context, *UpdateNotificationsIntegrationRequest) (*UpdateNotificationsIntegrationResponse, error)
	// deprecated Temporary streaming update endpoint
	UpdateNotificationsIntegrations(grpc.ClientStreamingServer[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]) error
	DeleteNotificationsIntegration(context.Context, *DeleteNotificationsIntegrationRequest) (*DeleteNotificationsIntegrationResponse, error)
	ListNotificationsIntegrations(*ListNotificationsIntegrationsRequest, grpc.ServerStreamingServer[ListNotificationsIntegrationsResponse]) error
	mustEmbedUnimplementedKesselNotificationsIntegrationServiceServer()
}

// UnimplementedKesselNotificationsIntegrationServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedKesselNotificationsIntegrationServiceServer struct{}

func (UnimplementedKesselNotificationsIntegrationServiceServer) CreateNotificationsIntegration(context.Context, *CreateNotificationsIntegrationRequest) (*CreateNotificationsIntegrationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateNotificationsIntegration not implemented")
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) UpdateNotificationsIntegration(context.Context, *UpdateNotificationsIntegrationRequest) (*UpdateNotificationsIntegrationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateNotificationsIntegration not implemented")
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) UpdateNotificationsIntegrations(grpc.ClientStreamingServer[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]) error {
	return status.Errorf(codes.Unimplemented, "method UpdateNotificationsIntegrations not implemented")
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) DeleteNotificationsIntegration(context.Context, *DeleteNotificationsIntegrationRequest) (*DeleteNotificationsIntegrationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteNotificationsIntegration not implemented")
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) ListNotificationsIntegrations(*ListNotificationsIntegrationsRequest, grpc.ServerStreamingServer[ListNotificationsIntegrationsResponse]) error {
	return status.Errorf(codes.Unimplemented, "method ListNotificationsIntegrations not implemented")
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) mustEmbedUnimplementedKesselNotificationsIntegrationServiceServer() {
}
func (UnimplementedKesselNotificationsIntegrationServiceServer) testEmbeddedByValue() {}

// UnsafeKesselNotificationsIntegrationServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KesselNotificationsIntegrationServiceServer will
// result in compilation errors.
type UnsafeKesselNotificationsIntegrationServiceServer interface {
	mustEmbedUnimplementedKesselNotificationsIntegrationServiceServer()
}

func RegisterKesselNotificationsIntegrationServiceServer(s grpc.ServiceRegistrar, srv KesselNotificationsIntegrationServiceServer) {
	// If the following call pancis, it indicates UnimplementedKesselNotificationsIntegrationServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&KesselNotificationsIntegrationService_ServiceDesc, srv)
}

func _KesselNotificationsIntegrationService_CreateNotificationsIntegration_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateNotificationsIntegrationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselNotificationsIntegrationServiceServer).CreateNotificationsIntegration(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselNotificationsIntegrationService_CreateNotificationsIntegration_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselNotificationsIntegrationServiceServer).CreateNotificationsIntegration(ctx, req.(*CreateNotificationsIntegrationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KesselNotificationsIntegrationService_UpdateNotificationsIntegration_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateNotificationsIntegrationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselNotificationsIntegrationServiceServer).UpdateNotificationsIntegration(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselNotificationsIntegrationService_UpdateNotificationsIntegration_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselNotificationsIntegrationServiceServer).UpdateNotificationsIntegration(ctx, req.(*UpdateNotificationsIntegrationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KesselNotificationsIntegrationService_UpdateNotificationsIntegrations_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(KesselNotificationsIntegrationServiceServer).UpdateNotificationsIntegrations(&grpc.GenericServerStream[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselNotificationsIntegrationService_UpdateNotificationsIntegrationsServer = grpc.ClientStreamingServer[UpdateNotificationsIntegrationsRequest, UpdateNotificationsIntegrationsResponse]

func _KesselNotificationsIntegrationService_DeleteNotificationsIntegration_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteNotificationsIntegrationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselNotificationsIntegrationServiceServer).DeleteNotificationsIntegration(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselNotificationsIntegrationService_DeleteNotificationsIntegration_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselNotificationsIntegrationServiceServer).DeleteNotificationsIntegration(ctx, req.(*DeleteNotificationsIntegrationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KesselNotificationsIntegrationService_ListNotificationsIntegrations_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ListNotificationsIntegrationsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(KesselNotificationsIntegrationServiceServer).ListNotificationsIntegrations(m, &grpc.GenericServerStream[ListNotificationsIntegrationsRequest, ListNotificationsIntegrationsResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type KesselNotificationsIntegrationService_ListNotificationsIntegrationsServer = grpc.ServerStreamingServer[ListNotificationsIntegrationsResponse]

// KesselNotificationsIntegrationService_ServiceDesc is the grpc.ServiceDesc for KesselNotificationsIntegrationService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KesselNotificationsIntegrationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService",
	HandlerType: (*KesselNotificationsIntegrationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateNotificationsIntegration",
			Handler:    _KesselNotificationsIntegrationService_CreateNotificationsIntegration_Handler,
		},
		{
			MethodName: "UpdateNotificationsIntegration",
			Handler:    _KesselNotificationsIntegrationService_UpdateNotificationsIntegration_Handler,
		},
		{
			MethodName: "DeleteNotificationsIntegration",
			Handler:    _KesselNotificationsIntegrationService_DeleteNotificationsIntegration_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "UpdateNotificationsIntegrations",
			Handler:       _KesselNotificationsIntegrationService_UpdateNotificationsIntegrations_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "ListNotificationsIntegrations",
			Handler:       _KesselNotificationsIntegrationService_ListNotificationsIntegrations_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "kessel/inventory/v1beta1/resources/notifications_integrations_service.proto",
}
