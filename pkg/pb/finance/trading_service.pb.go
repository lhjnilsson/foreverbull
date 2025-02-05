// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/finance/trading_service.proto

package finance

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetPortfolioRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetPortfolioRequest) Reset() {
	*x = GetPortfolioRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetPortfolioRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetPortfolioRequest) ProtoMessage() {}

func (x *GetPortfolioRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetPortfolioRequest.ProtoReflect.Descriptor instead.
func (*GetPortfolioRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{0}
}

type GetPortfolioResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Portfolio *Portfolio `protobuf:"bytes,1,opt,name=portfolio,proto3" json:"portfolio,omitempty"`
}

func (x *GetPortfolioResponse) Reset() {
	*x = GetPortfolioResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetPortfolioResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetPortfolioResponse) ProtoMessage() {}

func (x *GetPortfolioResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetPortfolioResponse.ProtoReflect.Descriptor instead.
func (*GetPortfolioResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{1}
}

func (x *GetPortfolioResponse) GetPortfolio() *Portfolio {
	if x != nil {
		return x.Portfolio
	}
	return nil
}

type GetOrdersRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetOrdersRequest) Reset() {
	*x = GetOrdersRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetOrdersRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetOrdersRequest) ProtoMessage() {}

func (x *GetOrdersRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetOrdersRequest.ProtoReflect.Descriptor instead.
func (*GetOrdersRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{2}
}

type GetOrdersResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Orders []*Order `protobuf:"bytes,1,rep,name=orders,proto3" json:"orders,omitempty"`
}

func (x *GetOrdersResponse) Reset() {
	*x = GetOrdersResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetOrdersResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetOrdersResponse) ProtoMessage() {}

func (x *GetOrdersResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetOrdersResponse.ProtoReflect.Descriptor instead.
func (*GetOrdersResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{3}
}

func (x *GetOrdersResponse) GetOrders() []*Order {
	if x != nil {
		return x.Orders
	}
	return nil
}

type PlaceOrderRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Order *Order `protobuf:"bytes,1,opt,name=order,proto3" json:"order,omitempty"`
}

func (x *PlaceOrderRequest) Reset() {
	*x = PlaceOrderRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlaceOrderRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlaceOrderRequest) ProtoMessage() {}

func (x *PlaceOrderRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlaceOrderRequest.ProtoReflect.Descriptor instead.
func (*PlaceOrderRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{4}
}

func (x *PlaceOrderRequest) GetOrder() *Order {
	if x != nil {
		return x.Order
	}
	return nil
}

type PlaceOrderResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PlaceOrderResponse) Reset() {
	*x = PlaceOrderResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_trading_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PlaceOrderResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PlaceOrderResponse) ProtoMessage() {}

func (x *PlaceOrderResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_trading_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PlaceOrderResponse.ProtoReflect.Descriptor instead.
func (*PlaceOrderResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_trading_service_proto_rawDescGZIP(), []int{5}
}

var File_foreverbull_finance_trading_service_proto protoreflect.FileDescriptor

var file_foreverbull_finance_trading_service_proto_rawDesc = []byte{
	0x0a, 0x29, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69,
	0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x74, 0x72, 0x61, 0x64, 0x69, 0x6e, 0x67, 0x5f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x13, 0x66, 0x6f, 0x72,
	0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65,
	0x1a, 0x21, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69,
	0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x15, 0x0a, 0x13, 0x47, 0x65, 0x74, 0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x54, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x50, 0x6f,
	0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x3c, 0x0a, 0x09, 0x70, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c,
	0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c,
	0x69, 0x6f, 0x52, 0x09, 0x70, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x22, 0x12, 0x0a,
	0x10, 0x47, 0x65, 0x74, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x22, 0x47, 0x0a, 0x11, 0x47, 0x65, 0x74, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x32, 0x0a, 0x06, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x4f, 0x72, 0x64,
	0x65, 0x72, 0x52, 0x06, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x22, 0x6e, 0x0a, 0x11, 0x50, 0x6c,
	0x61, 0x63, 0x65, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x59, 0x0a, 0x05, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e,
	0x61, 0x6e, 0x63, 0x65, 0x2e, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x42, 0x27, 0xba, 0x48, 0x24, 0xba,
	0x01, 0x1e, 0x0a, 0x0e, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72,
	0x65, 0x64, 0x1a, 0x0c, 0x74, 0x68, 0x69, 0x73, 0x20, 0x21, 0x3d, 0x20, 0x6e, 0x75, 0x6c, 0x6c,
	0xc8, 0x01, 0x01, 0x52, 0x05, 0x6f, 0x72, 0x64, 0x65, 0x72, 0x22, 0x14, 0x0a, 0x12, 0x50, 0x6c,
	0x61, 0x63, 0x65, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x32, 0xaf, 0x02, 0x0a, 0x07, 0x54, 0x72, 0x61, 0x64, 0x69, 0x6e, 0x67, 0x12, 0x65, 0x0a, 0x0c,
	0x47, 0x65, 0x74, 0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x12, 0x28, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e,
	0x63, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x29, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x47, 0x65, 0x74,
	0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x5c, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x12, 0x25, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66,
	0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x26, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65,
	0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x47, 0x65,
	0x74, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x5f, 0x0a, 0x0a, 0x50, 0x6c, 0x61, 0x63, 0x65, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x12,
	0x26, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69,
	0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x50, 0x6c, 0x61, 0x63, 0x65, 0x4f, 0x72, 0x64, 0x65, 0x72,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x27, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65,
	0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x50, 0x6c,
	0x61, 0x63, 0x65, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x00, 0x42, 0x32, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x6c, 0x68, 0x6a, 0x6e, 0x69, 0x6c, 0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65,
	0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x70, 0x62, 0x2f, 0x66,
	0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_finance_trading_service_proto_rawDescOnce sync.Once
	file_foreverbull_finance_trading_service_proto_rawDescData = file_foreverbull_finance_trading_service_proto_rawDesc
)

func file_foreverbull_finance_trading_service_proto_rawDescGZIP() []byte {
	file_foreverbull_finance_trading_service_proto_rawDescOnce.Do(func() {
		file_foreverbull_finance_trading_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_finance_trading_service_proto_rawDescData)
	})
	return file_foreverbull_finance_trading_service_proto_rawDescData
}

var file_foreverbull_finance_trading_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_foreverbull_finance_trading_service_proto_goTypes = []any{
	(*GetPortfolioRequest)(nil),  // 0: foreverbull.finance.GetPortfolioRequest
	(*GetPortfolioResponse)(nil), // 1: foreverbull.finance.GetPortfolioResponse
	(*GetOrdersRequest)(nil),     // 2: foreverbull.finance.GetOrdersRequest
	(*GetOrdersResponse)(nil),    // 3: foreverbull.finance.GetOrdersResponse
	(*PlaceOrderRequest)(nil),    // 4: foreverbull.finance.PlaceOrderRequest
	(*PlaceOrderResponse)(nil),   // 5: foreverbull.finance.PlaceOrderResponse
	(*Portfolio)(nil),            // 6: foreverbull.finance.Portfolio
	(*Order)(nil),                // 7: foreverbull.finance.Order
}
var file_foreverbull_finance_trading_service_proto_depIdxs = []int32{
	6, // 0: foreverbull.finance.GetPortfolioResponse.portfolio:type_name -> foreverbull.finance.Portfolio
	7, // 1: foreverbull.finance.GetOrdersResponse.orders:type_name -> foreverbull.finance.Order
	7, // 2: foreverbull.finance.PlaceOrderRequest.order:type_name -> foreverbull.finance.Order
	0, // 3: foreverbull.finance.Trading.GetPortfolio:input_type -> foreverbull.finance.GetPortfolioRequest
	2, // 4: foreverbull.finance.Trading.GetOrders:input_type -> foreverbull.finance.GetOrdersRequest
	4, // 5: foreverbull.finance.Trading.PlaceOrder:input_type -> foreverbull.finance.PlaceOrderRequest
	1, // 6: foreverbull.finance.Trading.GetPortfolio:output_type -> foreverbull.finance.GetPortfolioResponse
	3, // 7: foreverbull.finance.Trading.GetOrders:output_type -> foreverbull.finance.GetOrdersResponse
	5, // 8: foreverbull.finance.Trading.PlaceOrder:output_type -> foreverbull.finance.PlaceOrderResponse
	6, // [6:9] is the sub-list for method output_type
	3, // [3:6] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_foreverbull_finance_trading_service_proto_init() }
func file_foreverbull_finance_trading_service_proto_init() {
	if File_foreverbull_finance_trading_service_proto != nil {
		return
	}
	file_foreverbull_finance_finance_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_finance_trading_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*GetPortfolioRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_finance_trading_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*GetPortfolioResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_finance_trading_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*GetOrdersRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_finance_trading_service_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*GetOrdersResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_finance_trading_service_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*PlaceOrderRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_foreverbull_finance_trading_service_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*PlaceOrderResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_foreverbull_finance_trading_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_foreverbull_finance_trading_service_proto_goTypes,
		DependencyIndexes: file_foreverbull_finance_trading_service_proto_depIdxs,
		MessageInfos:      file_foreverbull_finance_trading_service_proto_msgTypes,
	}.Build()
	File_foreverbull_finance_trading_service_proto = out.File
	file_foreverbull_finance_trading_service_proto_rawDesc = nil
	file_foreverbull_finance_trading_service_proto_goTypes = nil
	file_foreverbull_finance_trading_service_proto_depIdxs = nil
}
