// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: kessel/inventory/v1beta1/relations/check.proto

package relations

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
	KesselCheckService_CheckForView_FullMethodName   = "/kessel.inventory.v1beta1.relations.KesselCheckService/CheckForView"
	KesselCheckService_CheckForUpdate_FullMethodName = "/kessel.inventory.v1beta1.relations.KesselCheckService/CheckForUpdate"
	KesselCheckService_CheckForCreate_FullMethodName = "/kessel.inventory.v1beta1.relations.KesselCheckService/CheckForCreate"
)

// KesselCheckServiceClient is the client API for KesselCheckService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type KesselCheckServiceClient interface {
	// Checks for the existence of a single Relationship
	// (a Relation between a Resource and a Subject or Subject Set).
	CheckForView(ctx context.Context, in *CheckForViewRequest, opts ...grpc.CallOption) (*CheckForViewResponse, error)
	CheckForUpdate(ctx context.Context, in *CheckForUpdateRequest, opts ...grpc.CallOption) (*CheckForUpdateResponse, error)
	CheckForCreate(ctx context.Context, in *CheckForCreateRequest, opts ...grpc.CallOption) (*CheckForCreateResponse, error)
}

type kesselCheckServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewKesselCheckServiceClient(cc grpc.ClientConnInterface) KesselCheckServiceClient {
	return &kesselCheckServiceClient{cc}
}

func (c *kesselCheckServiceClient) CheckForView(ctx context.Context, in *CheckForViewRequest, opts ...grpc.CallOption) (*CheckForViewResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckForViewResponse)
	err := c.cc.Invoke(ctx, KesselCheckService_CheckForView_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kesselCheckServiceClient) CheckForUpdate(ctx context.Context, in *CheckForUpdateRequest, opts ...grpc.CallOption) (*CheckForUpdateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckForUpdateResponse)
	err := c.cc.Invoke(ctx, KesselCheckService_CheckForUpdate_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kesselCheckServiceClient) CheckForCreate(ctx context.Context, in *CheckForCreateRequest, opts ...grpc.CallOption) (*CheckForCreateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CheckForCreateResponse)
	err := c.cc.Invoke(ctx, KesselCheckService_CheckForCreate_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// KesselCheckServiceServer is the server API for KesselCheckService service.
// All implementations must embed UnimplementedKesselCheckServiceServer
// for forward compatibility.
type KesselCheckServiceServer interface {
	// Checks for the existence of a single Relationship
	// (a Relation between a Resource and a Subject or Subject Set).
	CheckForView(context.Context, *CheckForViewRequest) (*CheckForViewResponse, error)
	CheckForUpdate(context.Context, *CheckForUpdateRequest) (*CheckForUpdateResponse, error)
	CheckForCreate(context.Context, *CheckForCreateRequest) (*CheckForCreateResponse, error)
	mustEmbedUnimplementedKesselCheckServiceServer()
}

// UnimplementedKesselCheckServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedKesselCheckServiceServer struct{}

func (UnimplementedKesselCheckServiceServer) CheckForView(context.Context, *CheckForViewRequest) (*CheckForViewResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckForView not implemented")
}
func (UnimplementedKesselCheckServiceServer) CheckForUpdate(context.Context, *CheckForUpdateRequest) (*CheckForUpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckForUpdate not implemented")
}
func (UnimplementedKesselCheckServiceServer) CheckForCreate(context.Context, *CheckForCreateRequest) (*CheckForCreateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CheckForCreate not implemented")
}
func (UnimplementedKesselCheckServiceServer) mustEmbedUnimplementedKesselCheckServiceServer() {}
func (UnimplementedKesselCheckServiceServer) testEmbeddedByValue()                            {}

// UnsafeKesselCheckServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to KesselCheckServiceServer will
// result in compilation errors.
type UnsafeKesselCheckServiceServer interface {
	mustEmbedUnimplementedKesselCheckServiceServer()
}

func RegisterKesselCheckServiceServer(s grpc.ServiceRegistrar, srv KesselCheckServiceServer) {
	// If the following call pancis, it indicates UnimplementedKesselCheckServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&KesselCheckService_ServiceDesc, srv)
}

func _KesselCheckService_CheckForView_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckForViewRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselCheckServiceServer).CheckForView(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselCheckService_CheckForView_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselCheckServiceServer).CheckForView(ctx, req.(*CheckForViewRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KesselCheckService_CheckForUpdate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckForUpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselCheckServiceServer).CheckForUpdate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselCheckService_CheckForUpdate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselCheckServiceServer).CheckForUpdate(ctx, req.(*CheckForUpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _KesselCheckService_CheckForCreate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckForCreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KesselCheckServiceServer).CheckForCreate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: KesselCheckService_CheckForCreate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KesselCheckServiceServer).CheckForCreate(ctx, req.(*CheckForCreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// KesselCheckService_ServiceDesc is the grpc.ServiceDesc for KesselCheckService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var KesselCheckService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "kessel.inventory.v1beta1.relations.KesselCheckService",
	HandlerType: (*KesselCheckServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CheckForView",
			Handler:    _KesselCheckService_CheckForView_Handler,
		},
		{
			MethodName: "CheckForUpdate",
			Handler:    _KesselCheckService_CheckForUpdate_Handler,
		},
		{
			MethodName: "CheckForCreate",
			Handler:    _KesselCheckService_CheckForCreate_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "kessel/inventory/v1beta1/relations/check.proto",
}
