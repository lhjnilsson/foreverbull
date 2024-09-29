// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.27.1
// source: foreverbull/service/worker_service.proto

package pb

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
	Worker_GetServiceInfo_FullMethodName     = "/foreverbull.service.Worker/GetServiceInfo"
	Worker_ConfigureExecution_FullMethodName = "/foreverbull.service.Worker/ConfigureExecution"
	Worker_RunExecution_FullMethodName       = "/foreverbull.service.Worker/RunExecution"
)

// WorkerClient is the client API for Worker service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type WorkerClient interface {
	GetServiceInfo(ctx context.Context, in *GetServiceInfoRequest, opts ...grpc.CallOption) (*GetServiceInfoResponse, error)
	ConfigureExecution(ctx context.Context, in *ConfigureExecutionRequest, opts ...grpc.CallOption) (*ConfigureExecutionResponse, error)
	RunExecution(ctx context.Context, in *RunExecutionRequest, opts ...grpc.CallOption) (*RunExecutionResponse, error)
}

type workerClient struct {
	cc grpc.ClientConnInterface
}

func NewWorkerClient(cc grpc.ClientConnInterface) WorkerClient {
	return &workerClient{cc}
}

func (c *workerClient) GetServiceInfo(ctx context.Context, in *GetServiceInfoRequest, opts ...grpc.CallOption) (*GetServiceInfoResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetServiceInfoResponse)
	err := c.cc.Invoke(ctx, Worker_GetServiceInfo_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *workerClient) ConfigureExecution(ctx context.Context, in *ConfigureExecutionRequest, opts ...grpc.CallOption) (*ConfigureExecutionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConfigureExecutionResponse)
	err := c.cc.Invoke(ctx, Worker_ConfigureExecution_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *workerClient) RunExecution(ctx context.Context, in *RunExecutionRequest, opts ...grpc.CallOption) (*RunExecutionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RunExecutionResponse)
	err := c.cc.Invoke(ctx, Worker_RunExecution_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// WorkerServer is the server API for Worker service.
// All implementations must embed UnimplementedWorkerServer
// for forward compatibility.
type WorkerServer interface {
	GetServiceInfo(context.Context, *GetServiceInfoRequest) (*GetServiceInfoResponse, error)
	ConfigureExecution(context.Context, *ConfigureExecutionRequest) (*ConfigureExecutionResponse, error)
	RunExecution(context.Context, *RunExecutionRequest) (*RunExecutionResponse, error)
	mustEmbedUnimplementedWorkerServer()
}

// UnimplementedWorkerServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedWorkerServer struct{}

func (UnimplementedWorkerServer) GetServiceInfo(context.Context, *GetServiceInfoRequest) (*GetServiceInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetServiceInfo not implemented")
}
func (UnimplementedWorkerServer) ConfigureExecution(context.Context, *ConfigureExecutionRequest) (*ConfigureExecutionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ConfigureExecution not implemented")
}
func (UnimplementedWorkerServer) RunExecution(context.Context, *RunExecutionRequest) (*RunExecutionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunExecution not implemented")
}
func (UnimplementedWorkerServer) mustEmbedUnimplementedWorkerServer() {}
func (UnimplementedWorkerServer) testEmbeddedByValue()                {}

// UnsafeWorkerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to WorkerServer will
// result in compilation errors.
type UnsafeWorkerServer interface {
	mustEmbedUnimplementedWorkerServer()
}

func RegisterWorkerServer(s grpc.ServiceRegistrar, srv WorkerServer) {
	// If the following call pancis, it indicates UnimplementedWorkerServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Worker_ServiceDesc, srv)
}

func _Worker_GetServiceInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetServiceInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerServer).GetServiceInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Worker_GetServiceInfo_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerServer).GetServiceInfo(ctx, req.(*GetServiceInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Worker_ConfigureExecution_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigureExecutionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerServer).ConfigureExecution(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Worker_ConfigureExecution_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerServer).ConfigureExecution(ctx, req.(*ConfigureExecutionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Worker_RunExecution_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RunExecutionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(WorkerServer).RunExecution(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Worker_RunExecution_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(WorkerServer).RunExecution(ctx, req.(*RunExecutionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Worker_ServiceDesc is the grpc.ServiceDesc for Worker service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Worker_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "foreverbull.service.Worker",
	HandlerType: (*WorkerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetServiceInfo",
			Handler:    _Worker_GetServiceInfo_Handler,
		},
		{
			MethodName: "ConfigureExecution",
			Handler:    _Worker_ConfigureExecution_Handler,
		},
		{
			MethodName: "RunExecution",
			Handler:    _Worker_RunExecution_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "foreverbull/service/worker_service.proto",
}