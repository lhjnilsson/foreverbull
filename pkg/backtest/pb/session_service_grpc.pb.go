// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.27.1
// source: foreverbull/backtest/session_service.proto

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
	SessionServicer_CreateExecution_FullMethodName = "/foreverbull.backtest.SessionServicer/CreateExecution"
	SessionServicer_RunExecution_FullMethodName    = "/foreverbull.backtest.SessionServicer/RunExecution"
	SessionServicer_GetExecution_FullMethodName    = "/foreverbull.backtest.SessionServicer/GetExecution"
	SessionServicer_StopServer_FullMethodName      = "/foreverbull.backtest.SessionServicer/StopServer"
)

// SessionServicerClient is the client API for SessionServicer service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SessionServicerClient interface {
	CreateExecution(ctx context.Context, in *CreateExecutionRequest, opts ...grpc.CallOption) (*CreateExecutionResponse, error)
	RunExecution(ctx context.Context, in *RunExecutionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[RunExecutionResponse], error)
	GetExecution(ctx context.Context, in *GetExecutionRequest, opts ...grpc.CallOption) (*GetExecutionResponse, error)
	StopServer(ctx context.Context, in *StopServerRequest, opts ...grpc.CallOption) (*StopServerResponse, error)
}

type sessionServicerClient struct {
	cc grpc.ClientConnInterface
}

func NewSessionServicerClient(cc grpc.ClientConnInterface) SessionServicerClient {
	return &sessionServicerClient{cc}
}

func (c *sessionServicerClient) CreateExecution(ctx context.Context, in *CreateExecutionRequest, opts ...grpc.CallOption) (*CreateExecutionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateExecutionResponse)
	err := c.cc.Invoke(ctx, SessionServicer_CreateExecution_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionServicerClient) RunExecution(ctx context.Context, in *RunExecutionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[RunExecutionResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &SessionServicer_ServiceDesc.Streams[0], SessionServicer_RunExecution_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[RunExecutionRequest, RunExecutionResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SessionServicer_RunExecutionClient = grpc.ServerStreamingClient[RunExecutionResponse]

func (c *sessionServicerClient) GetExecution(ctx context.Context, in *GetExecutionRequest, opts ...grpc.CallOption) (*GetExecutionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetExecutionResponse)
	err := c.cc.Invoke(ctx, SessionServicer_GetExecution_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sessionServicerClient) StopServer(ctx context.Context, in *StopServerRequest, opts ...grpc.CallOption) (*StopServerResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(StopServerResponse)
	err := c.cc.Invoke(ctx, SessionServicer_StopServer_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SessionServicerServer is the server API for SessionServicer service.
// All implementations must embed UnimplementedSessionServicerServer
// for forward compatibility.
type SessionServicerServer interface {
	CreateExecution(context.Context, *CreateExecutionRequest) (*CreateExecutionResponse, error)
	RunExecution(*RunExecutionRequest, grpc.ServerStreamingServer[RunExecutionResponse]) error
	GetExecution(context.Context, *GetExecutionRequest) (*GetExecutionResponse, error)
	StopServer(context.Context, *StopServerRequest) (*StopServerResponse, error)
	mustEmbedUnimplementedSessionServicerServer()
}

// UnimplementedSessionServicerServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSessionServicerServer struct{}

func (UnimplementedSessionServicerServer) CreateExecution(context.Context, *CreateExecutionRequest) (*CreateExecutionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateExecution not implemented")
}
func (UnimplementedSessionServicerServer) RunExecution(*RunExecutionRequest, grpc.ServerStreamingServer[RunExecutionResponse]) error {
	return status.Errorf(codes.Unimplemented, "method RunExecution not implemented")
}
func (UnimplementedSessionServicerServer) GetExecution(context.Context, *GetExecutionRequest) (*GetExecutionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetExecution not implemented")
}
func (UnimplementedSessionServicerServer) StopServer(context.Context, *StopServerRequest) (*StopServerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopServer not implemented")
}
func (UnimplementedSessionServicerServer) mustEmbedUnimplementedSessionServicerServer() {}
func (UnimplementedSessionServicerServer) testEmbeddedByValue()                         {}

// UnsafeSessionServicerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SessionServicerServer will
// result in compilation errors.
type UnsafeSessionServicerServer interface {
	mustEmbedUnimplementedSessionServicerServer()
}

func RegisterSessionServicerServer(s grpc.ServiceRegistrar, srv SessionServicerServer) {
	// If the following call pancis, it indicates UnimplementedSessionServicerServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SessionServicer_ServiceDesc, srv)
}

func _SessionServicer_CreateExecution_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateExecutionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionServicerServer).CreateExecution(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SessionServicer_CreateExecution_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionServicerServer).CreateExecution(ctx, req.(*CreateExecutionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionServicer_RunExecution_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(RunExecutionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SessionServicerServer).RunExecution(m, &grpc.GenericServerStream[RunExecutionRequest, RunExecutionResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SessionServicer_RunExecutionServer = grpc.ServerStreamingServer[RunExecutionResponse]

func _SessionServicer_GetExecution_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetExecutionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionServicerServer).GetExecution(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SessionServicer_GetExecution_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionServicerServer).GetExecution(ctx, req.(*GetExecutionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SessionServicer_StopServer_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopServerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SessionServicerServer).StopServer(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SessionServicer_StopServer_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SessionServicerServer).StopServer(ctx, req.(*StopServerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SessionServicer_ServiceDesc is the grpc.ServiceDesc for SessionServicer service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SessionServicer_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "foreverbull.backtest.SessionServicer",
	HandlerType: (*SessionServicerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateExecution",
			Handler:    _SessionServicer_CreateExecution_Handler,
		},
		{
			MethodName: "GetExecution",
			Handler:    _SessionServicer_GetExecution_Handler,
		},
		{
			MethodName: "StopServer",
			Handler:    _SessionServicer_StopServer_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "RunExecution",
			Handler:       _SessionServicer_RunExecution_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "foreverbull/backtest/session_service.proto",
}