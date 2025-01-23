// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.27.1
// source: foreverbull/backtest/ingestion_service.proto

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
	IngestionServicer_GetCurrentIngestion_FullMethodName = "/foreverbull.backtest.IngestionServicer/GetCurrentIngestion"
	IngestionServicer_UpdateIngestion_FullMethodName     = "/foreverbull.backtest.IngestionServicer/UpdateIngestion"
)

// IngestionServicerClient is the client API for IngestionServicer service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type IngestionServicerClient interface {
	GetCurrentIngestion(ctx context.Context, in *GetCurrentIngestionRequest, opts ...grpc.CallOption) (*GetCurrentIngestionResponse, error)
	UpdateIngestion(ctx context.Context, in *UpdateIngestionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[UpdateIngestionResponse], error)
}

type ingestionServicerClient struct {
	cc grpc.ClientConnInterface
}

func NewIngestionServicerClient(cc grpc.ClientConnInterface) IngestionServicerClient {
	return &ingestionServicerClient{cc}
}

func (c *ingestionServicerClient) GetCurrentIngestion(ctx context.Context, in *GetCurrentIngestionRequest, opts ...grpc.CallOption) (*GetCurrentIngestionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetCurrentIngestionResponse)
	err := c.cc.Invoke(ctx, IngestionServicer_GetCurrentIngestion_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *ingestionServicerClient) UpdateIngestion(ctx context.Context, in *UpdateIngestionRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[UpdateIngestionResponse], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &IngestionServicer_ServiceDesc.Streams[0], IngestionServicer_UpdateIngestion_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[UpdateIngestionRequest, UpdateIngestionResponse]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type IngestionServicer_UpdateIngestionClient = grpc.ServerStreamingClient[UpdateIngestionResponse]

// IngestionServicerServer is the server API for IngestionServicer service.
// All implementations must embed UnimplementedIngestionServicerServer
// for forward compatibility.
type IngestionServicerServer interface {
	GetCurrentIngestion(context.Context, *GetCurrentIngestionRequest) (*GetCurrentIngestionResponse, error)
	UpdateIngestion(*UpdateIngestionRequest, grpc.ServerStreamingServer[UpdateIngestionResponse]) error
	mustEmbedUnimplementedIngestionServicerServer()
}

// UnimplementedIngestionServicerServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedIngestionServicerServer struct{}

func (UnimplementedIngestionServicerServer) GetCurrentIngestion(context.Context, *GetCurrentIngestionRequest) (*GetCurrentIngestionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCurrentIngestion not implemented")
}
func (UnimplementedIngestionServicerServer) UpdateIngestion(*UpdateIngestionRequest, grpc.ServerStreamingServer[UpdateIngestionResponse]) error {
	return status.Errorf(codes.Unimplemented, "method UpdateIngestion not implemented")
}
func (UnimplementedIngestionServicerServer) mustEmbedUnimplementedIngestionServicerServer() {}
func (UnimplementedIngestionServicerServer) testEmbeddedByValue()                           {}

// UnsafeIngestionServicerServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to IngestionServicerServer will
// result in compilation errors.
type UnsafeIngestionServicerServer interface {
	mustEmbedUnimplementedIngestionServicerServer()
}

func RegisterIngestionServicerServer(s grpc.ServiceRegistrar, srv IngestionServicerServer) {
	// If the following call pancis, it indicates UnimplementedIngestionServicerServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&IngestionServicer_ServiceDesc, srv)
}

func _IngestionServicer_GetCurrentIngestion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetCurrentIngestionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(IngestionServicerServer).GetCurrentIngestion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: IngestionServicer_GetCurrentIngestion_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(IngestionServicerServer).GetCurrentIngestion(ctx, req.(*GetCurrentIngestionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _IngestionServicer_UpdateIngestion_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(UpdateIngestionRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(IngestionServicerServer).UpdateIngestion(m, &grpc.GenericServerStream[UpdateIngestionRequest, UpdateIngestionResponse]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type IngestionServicer_UpdateIngestionServer = grpc.ServerStreamingServer[UpdateIngestionResponse]

// IngestionServicer_ServiceDesc is the grpc.ServiceDesc for IngestionServicer service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var IngestionServicer_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "foreverbull.backtest.IngestionServicer",
	HandlerType: (*IngestionServicerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetCurrentIngestion",
			Handler:    _IngestionServicer_GetCurrentIngestion_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "UpdateIngestion",
			Handler:       _IngestionServicer_UpdateIngestion_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "foreverbull/backtest/ingestion_service.proto",
}
