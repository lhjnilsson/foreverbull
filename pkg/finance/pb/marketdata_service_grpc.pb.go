// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v5.27.1
// source: foreverbull/finance/marketdata_service.proto

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
	Marketdata_GetAsset_FullMethodName               = "/foreverbull.finance.Marketdata/GetAsset"
	Marketdata_GetIndex_FullMethodName               = "/foreverbull.finance.Marketdata/GetIndex"
	Marketdata_DownloadHistoricalData_FullMethodName = "/foreverbull.finance.Marketdata/DownloadHistoricalData"
)

// MarketdataClient is the client API for Marketdata service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type MarketdataClient interface {
	GetAsset(ctx context.Context, in *GetAssetRequest, opts ...grpc.CallOption) (*GetAssetResponse, error)
	GetIndex(ctx context.Context, in *GetIndexRequest, opts ...grpc.CallOption) (*GetIndexResponse, error)
	DownloadHistoricalData(ctx context.Context, in *DownloadHistoricalDataRequest, opts ...grpc.CallOption) (*DownloadHistoricalDataResponse, error)
}

type marketdataClient struct {
	cc grpc.ClientConnInterface
}

func NewMarketdataClient(cc grpc.ClientConnInterface) MarketdataClient {
	return &marketdataClient{cc}
}

func (c *marketdataClient) GetAsset(ctx context.Context, in *GetAssetRequest, opts ...grpc.CallOption) (*GetAssetResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetAssetResponse)
	err := c.cc.Invoke(ctx, Marketdata_GetAsset_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *marketdataClient) GetIndex(ctx context.Context, in *GetIndexRequest, opts ...grpc.CallOption) (*GetIndexResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(GetIndexResponse)
	err := c.cc.Invoke(ctx, Marketdata_GetIndex_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *marketdataClient) DownloadHistoricalData(ctx context.Context, in *DownloadHistoricalDataRequest, opts ...grpc.CallOption) (*DownloadHistoricalDataResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DownloadHistoricalDataResponse)
	err := c.cc.Invoke(ctx, Marketdata_DownloadHistoricalData_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MarketdataServer is the server API for Marketdata service.
// All implementations must embed UnimplementedMarketdataServer
// for forward compatibility.
type MarketdataServer interface {
	GetAsset(context.Context, *GetAssetRequest) (*GetAssetResponse, error)
	GetIndex(context.Context, *GetIndexRequest) (*GetIndexResponse, error)
	DownloadHistoricalData(context.Context, *DownloadHistoricalDataRequest) (*DownloadHistoricalDataResponse, error)
	mustEmbedUnimplementedMarketdataServer()
}

// UnimplementedMarketdataServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedMarketdataServer struct{}

func (UnimplementedMarketdataServer) GetAsset(context.Context, *GetAssetRequest) (*GetAssetResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAsset not implemented")
}
func (UnimplementedMarketdataServer) GetIndex(context.Context, *GetIndexRequest) (*GetIndexResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetIndex not implemented")
}
func (UnimplementedMarketdataServer) DownloadHistoricalData(context.Context, *DownloadHistoricalDataRequest) (*DownloadHistoricalDataResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadHistoricalData not implemented")
}
func (UnimplementedMarketdataServer) mustEmbedUnimplementedMarketdataServer() {}
func (UnimplementedMarketdataServer) testEmbeddedByValue()                    {}

// UnsafeMarketdataServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to MarketdataServer will
// result in compilation errors.
type UnsafeMarketdataServer interface {
	mustEmbedUnimplementedMarketdataServer()
}

func RegisterMarketdataServer(s grpc.ServiceRegistrar, srv MarketdataServer) {
	// If the following call pancis, it indicates UnimplementedMarketdataServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Marketdata_ServiceDesc, srv)
}

func _Marketdata_GetAsset_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAssetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MarketdataServer).GetAsset(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Marketdata_GetAsset_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MarketdataServer).GetAsset(ctx, req.(*GetAssetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Marketdata_GetIndex_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetIndexRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MarketdataServer).GetIndex(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Marketdata_GetIndex_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MarketdataServer).GetIndex(ctx, req.(*GetIndexRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Marketdata_DownloadHistoricalData_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DownloadHistoricalDataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MarketdataServer).DownloadHistoricalData(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Marketdata_DownloadHistoricalData_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MarketdataServer).DownloadHistoricalData(ctx, req.(*DownloadHistoricalDataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Marketdata_ServiceDesc is the grpc.ServiceDesc for Marketdata service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Marketdata_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "foreverbull.finance.Marketdata",
	HandlerType: (*MarketdataServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetAsset",
			Handler:    _Marketdata_GetAsset_Handler,
		},
		{
			MethodName: "GetIndex",
			Handler:    _Marketdata_GetIndex_Handler,
		},
		{
			MethodName: "DownloadHistoricalData",
			Handler:    _Marketdata_DownloadHistoricalData_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "foreverbull/finance/marketdata_service.proto",
}
