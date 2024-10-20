// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/finance/finance.proto

package pb

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Asset struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol string `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
	Name   string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Asset) Reset() {
	*x = Asset{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_finance_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Asset) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Asset) ProtoMessage() {}

func (x *Asset) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_finance_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Asset.ProtoReflect.Descriptor instead.
func (*Asset) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_finance_proto_rawDescGZIP(), []int{0}
}

func (x *Asset) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

func (x *Asset) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type OHLC struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol    string                 `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
	Timestamp *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	Open      float64                `protobuf:"fixed64,3,opt,name=open,proto3" json:"open,omitempty"`
	High      float64                `protobuf:"fixed64,4,opt,name=high,proto3" json:"high,omitempty"`
	Low       float64                `protobuf:"fixed64,5,opt,name=low,proto3" json:"low,omitempty"`
	Close     float64                `protobuf:"fixed64,6,opt,name=close,proto3" json:"close,omitempty"`
	Volume    int32                  `protobuf:"varint,7,opt,name=volume,proto3" json:"volume,omitempty"`
}

func (x *OHLC) Reset() {
	*x = OHLC{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_finance_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OHLC) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OHLC) ProtoMessage() {}

func (x *OHLC) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_finance_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OHLC.ProtoReflect.Descriptor instead.
func (*OHLC) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_finance_proto_rawDescGZIP(), []int{1}
}

func (x *OHLC) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

func (x *OHLC) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *OHLC) GetOpen() float64 {
	if x != nil {
		return x.Open
	}
	return 0
}

func (x *OHLC) GetHigh() float64 {
	if x != nil {
		return x.High
	}
	return 0
}

func (x *OHLC) GetLow() float64 {
	if x != nil {
		return x.Low
	}
	return 0
}

func (x *OHLC) GetClose() float64 {
	if x != nil {
		return x.Close
	}
	return 0
}

func (x *OHLC) GetVolume() int32 {
	if x != nil {
		return x.Volume
	}
	return 0
}

type Position struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol        string                 `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
	Amount        int32                  `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
	CostBasis     float64                `protobuf:"fixed64,3,opt,name=cost_basis,json=costBasis,proto3" json:"cost_basis,omitempty"`
	LastSalePrice float64                `protobuf:"fixed64,4,opt,name=last_sale_price,json=lastSalePrice,proto3" json:"last_sale_price,omitempty"`
	LastSaleDate  *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=last_sale_date,json=lastSaleDate,proto3" json:"last_sale_date,omitempty"`
}

func (x *Position) Reset() {
	*x = Position{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_finance_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Position) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Position) ProtoMessage() {}

func (x *Position) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_finance_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Position.ProtoReflect.Descriptor instead.
func (*Position) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_finance_proto_rawDescGZIP(), []int{2}
}

func (x *Position) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

func (x *Position) GetAmount() int32 {
	if x != nil {
		return x.Amount
	}
	return 0
}

func (x *Position) GetCostBasis() float64 {
	if x != nil {
		return x.CostBasis
	}
	return 0
}

func (x *Position) GetLastSalePrice() float64 {
	if x != nil {
		return x.LastSalePrice
	}
	return 0
}

func (x *Position) GetLastSaleDate() *timestamppb.Timestamp {
	if x != nil {
		return x.LastSaleDate
	}
	return nil
}

type Portfolio struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Timestamp         *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	CashFlow          float64                `protobuf:"fixed64,2,opt,name=cash_flow,json=cashFlow,proto3" json:"cash_flow,omitempty"`
	StartingCash      float64                `protobuf:"fixed64,3,opt,name=starting_cash,json=startingCash,proto3" json:"starting_cash,omitempty"`
	PortfolioValue    float64                `protobuf:"fixed64,4,opt,name=portfolio_value,json=portfolioValue,proto3" json:"portfolio_value,omitempty"`
	Pnl               float64                `protobuf:"fixed64,5,opt,name=pnl,proto3" json:"pnl,omitempty"`
	Returns           float64                `protobuf:"fixed64,6,opt,name=returns,proto3" json:"returns,omitempty"`
	Cash              float64                `protobuf:"fixed64,7,opt,name=cash,proto3" json:"cash,omitempty"`
	PositionsValue    float64                `protobuf:"fixed64,8,opt,name=positions_value,json=positionsValue,proto3" json:"positions_value,omitempty"`
	PositionsExposure float64                `protobuf:"fixed64,9,opt,name=positions_exposure,json=positionsExposure,proto3" json:"positions_exposure,omitempty"`
	Positions         []*Position            `protobuf:"bytes,10,rep,name=positions,proto3" json:"positions,omitempty"`
}

func (x *Portfolio) Reset() {
	*x = Portfolio{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_finance_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Portfolio) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Portfolio) ProtoMessage() {}

func (x *Portfolio) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_finance_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Portfolio.ProtoReflect.Descriptor instead.
func (*Portfolio) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_finance_proto_rawDescGZIP(), []int{3}
}

func (x *Portfolio) GetTimestamp() *timestamppb.Timestamp {
	if x != nil {
		return x.Timestamp
	}
	return nil
}

func (x *Portfolio) GetCashFlow() float64 {
	if x != nil {
		return x.CashFlow
	}
	return 0
}

func (x *Portfolio) GetStartingCash() float64 {
	if x != nil {
		return x.StartingCash
	}
	return 0
}

func (x *Portfolio) GetPortfolioValue() float64 {
	if x != nil {
		return x.PortfolioValue
	}
	return 0
}

func (x *Portfolio) GetPnl() float64 {
	if x != nil {
		return x.Pnl
	}
	return 0
}

func (x *Portfolio) GetReturns() float64 {
	if x != nil {
		return x.Returns
	}
	return 0
}

func (x *Portfolio) GetCash() float64 {
	if x != nil {
		return x.Cash
	}
	return 0
}

func (x *Portfolio) GetPositionsValue() float64 {
	if x != nil {
		return x.PositionsValue
	}
	return 0
}

func (x *Portfolio) GetPositionsExposure() float64 {
	if x != nil {
		return x.PositionsExposure
	}
	return 0
}

func (x *Portfolio) GetPositions() []*Position {
	if x != nil {
		return x.Positions
	}
	return nil
}

type Order struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol string `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
	Amount int32  `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
}

func (x *Order) Reset() {
	*x = Order{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_finance_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Order) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Order) ProtoMessage() {}

func (x *Order) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_finance_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Order.ProtoReflect.Descriptor instead.
func (*Order) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_finance_proto_rawDescGZIP(), []int{4}
}

func (x *Order) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

func (x *Order) GetAmount() int32 {
	if x != nil {
		return x.Amount
	}
	return 0
}

var File_foreverbull_finance_finance_proto protoreflect.FileDescriptor

var file_foreverbull_finance_finance_proto_rawDesc = []byte{
	0x0a, 0x21, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69,
	0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x13, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c,
	0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74,
	0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x33, 0x0a, 0x05, 0x41, 0x73, 0x73,
	0x65, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0xc0,
	0x01, 0x0a, 0x04, 0x4f, 0x48, 0x4c, 0x43, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x12,
	0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09,
	0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x6f, 0x70, 0x65,
	0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01, 0x52, 0x04, 0x6f, 0x70, 0x65, 0x6e, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x69, 0x67, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x52, 0x04, 0x68, 0x69, 0x67,
	0x68, 0x12, 0x10, 0x0a, 0x03, 0x6c, 0x6f, 0x77, 0x18, 0x05, 0x20, 0x01, 0x28, 0x01, 0x52, 0x03,
	0x6c, 0x6f, 0x77, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x05, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x76, 0x6f, 0x6c,
	0x75, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x76, 0x6f, 0x6c, 0x75, 0x6d,
	0x65, 0x22, 0xc3, 0x01, 0x0a, 0x08, 0x50, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x12, 0x16, 0x0a, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1d,
	0x0a, 0x0a, 0x63, 0x6f, 0x73, 0x74, 0x5f, 0x62, 0x61, 0x73, 0x69, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x09, 0x63, 0x6f, 0x73, 0x74, 0x42, 0x61, 0x73, 0x69, 0x73, 0x12, 0x26, 0x0a,
	0x0f, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x73, 0x61, 0x6c, 0x65, 0x5f, 0x70, 0x72, 0x69, 0x63, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0d, 0x6c, 0x61, 0x73, 0x74, 0x53, 0x61, 0x6c, 0x65,
	0x50, 0x72, 0x69, 0x63, 0x65, 0x12, 0x40, 0x0a, 0x0e, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x73, 0x61,
	0x6c, 0x65, 0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0c, 0x6c, 0x61, 0x73, 0x74, 0x53,
	0x61, 0x6c, 0x65, 0x44, 0x61, 0x74, 0x65, 0x22, 0x85, 0x03, 0x0a, 0x09, 0x50, 0x6f, 0x72, 0x74,
	0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x12, 0x38, 0x0a, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x12,
	0x1b, 0x0a, 0x09, 0x63, 0x61, 0x73, 0x68, 0x5f, 0x66, 0x6c, 0x6f, 0x77, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x01, 0x52, 0x08, 0x63, 0x61, 0x73, 0x68, 0x46, 0x6c, 0x6f, 0x77, 0x12, 0x23, 0x0a, 0x0d,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x63, 0x61, 0x73, 0x68, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x0c, 0x73, 0x74, 0x61, 0x72, 0x74, 0x69, 0x6e, 0x67, 0x43, 0x61, 0x73,
	0x68, 0x12, 0x27, 0x0a, 0x0f, 0x70, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x5f, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x52, 0x0e, 0x70, 0x6f, 0x72, 0x74,
	0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x70, 0x6e,
	0x6c, 0x18, 0x05, 0x20, 0x01, 0x28, 0x01, 0x52, 0x03, 0x70, 0x6e, 0x6c, 0x12, 0x18, 0x0a, 0x07,
	0x72, 0x65, 0x74, 0x75, 0x72, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x01, 0x52, 0x07, 0x72,
	0x65, 0x74, 0x75, 0x72, 0x6e, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x61, 0x73, 0x68, 0x18, 0x07,
	0x20, 0x01, 0x28, 0x01, 0x52, 0x04, 0x63, 0x61, 0x73, 0x68, 0x12, 0x27, 0x0a, 0x0f, 0x70, 0x6f,
	0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x08, 0x20,
	0x01, 0x28, 0x01, 0x52, 0x0e, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x12, 0x2d, 0x0a, 0x12, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x5f, 0x65, 0x78, 0x70, 0x6f, 0x73, 0x75, 0x72, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x01, 0x52,
	0x11, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x45, 0x78, 0x70, 0x6f, 0x73, 0x75,
	0x72, 0x65, 0x12, 0x3b, 0x0a, 0x09, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18,
	0x0a, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62,
	0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x50, 0x6f, 0x73, 0x69,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x22,
	0x37, 0x0a, 0x05, 0x4f, 0x72, 0x64, 0x65, 0x72, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62,
	0x6f, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c,
	0x12, 0x16, 0x0a, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x06, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x42, 0x32, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x68, 0x6a, 0x6e, 0x69, 0x6c, 0x73, 0x73, 0x6f,
	0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_finance_finance_proto_rawDescOnce sync.Once
	file_foreverbull_finance_finance_proto_rawDescData = file_foreverbull_finance_finance_proto_rawDesc
)

func file_foreverbull_finance_finance_proto_rawDescGZIP() []byte {
	file_foreverbull_finance_finance_proto_rawDescOnce.Do(func() {
		file_foreverbull_finance_finance_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_finance_finance_proto_rawDescData)
	})
	return file_foreverbull_finance_finance_proto_rawDescData
}

var file_foreverbull_finance_finance_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_foreverbull_finance_finance_proto_goTypes = []any{
	(*Asset)(nil),                 // 0: foreverbull.finance.Asset
	(*OHLC)(nil),                  // 1: foreverbull.finance.OHLC
	(*Position)(nil),              // 2: foreverbull.finance.Position
	(*Portfolio)(nil),             // 3: foreverbull.finance.Portfolio
	(*Order)(nil),                 // 4: foreverbull.finance.Order
	(*timestamppb.Timestamp)(nil), // 5: google.protobuf.Timestamp
}
var file_foreverbull_finance_finance_proto_depIdxs = []int32{
	5, // 0: foreverbull.finance.OHLC.timestamp:type_name -> google.protobuf.Timestamp
	5, // 1: foreverbull.finance.Position.last_sale_date:type_name -> google.protobuf.Timestamp
	5, // 2: foreverbull.finance.Portfolio.timestamp:type_name -> google.protobuf.Timestamp
	2, // 3: foreverbull.finance.Portfolio.positions:type_name -> foreverbull.finance.Position
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_foreverbull_finance_finance_proto_init() }
func file_foreverbull_finance_finance_proto_init() {
	if File_foreverbull_finance_finance_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_finance_finance_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Asset); i {
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
		file_foreverbull_finance_finance_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*OHLC); i {
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
		file_foreverbull_finance_finance_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*Position); i {
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
		file_foreverbull_finance_finance_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*Portfolio); i {
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
		file_foreverbull_finance_finance_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*Order); i {
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
			RawDescriptor: file_foreverbull_finance_finance_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_foreverbull_finance_finance_proto_goTypes,
		DependencyIndexes: file_foreverbull_finance_finance_proto_depIdxs,
		MessageInfos:      file_foreverbull_finance_finance_proto_msgTypes,
	}.Build()
	File_foreverbull_finance_finance_proto = out.File
	file_foreverbull_finance_finance_proto_rawDesc = nil
	file_foreverbull_finance_finance_proto_goTypes = nil
	file_foreverbull_finance_finance_proto_depIdxs = nil
}
