// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/finance/marketdata_service.proto

package pb

import (
	pb "github.com/lhjnilsson/foreverbull/internal/pb"
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

type GetAssetRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol string `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
}

func (x *GetAssetRequest) Reset() {
	*x = GetAssetRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAssetRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAssetRequest) ProtoMessage() {}

func (x *GetAssetRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAssetRequest.ProtoReflect.Descriptor instead.
func (*GetAssetRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{0}
}

func (x *GetAssetRequest) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

type GetAssetResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Asset *Asset `protobuf:"bytes,1,opt,name=asset,proto3" json:"asset,omitempty"`
}

func (x *GetAssetResponse) Reset() {
	*x = GetAssetResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetAssetResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetAssetResponse) ProtoMessage() {}

func (x *GetAssetResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetAssetResponse.ProtoReflect.Descriptor instead.
func (*GetAssetResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{1}
}

func (x *GetAssetResponse) GetAsset() *Asset {
	if x != nil {
		return x.Asset
	}
	return nil
}

type GetIndexRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbol string `protobuf:"bytes,1,opt,name=symbol,proto3" json:"symbol,omitempty"`
}

func (x *GetIndexRequest) Reset() {
	*x = GetIndexRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetIndexRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetIndexRequest) ProtoMessage() {}

func (x *GetIndexRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetIndexRequest.ProtoReflect.Descriptor instead.
func (*GetIndexRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{2}
}

func (x *GetIndexRequest) GetSymbol() string {
	if x != nil {
		return x.Symbol
	}
	return ""
}

type GetIndexResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Assets []*Asset `protobuf:"bytes,1,rep,name=assets,proto3" json:"assets,omitempty"`
}

func (x *GetIndexResponse) Reset() {
	*x = GetIndexResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetIndexResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetIndexResponse) ProtoMessage() {}

func (x *GetIndexResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetIndexResponse.ProtoReflect.Descriptor instead.
func (*GetIndexResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{3}
}

func (x *GetIndexResponse) GetAssets() []*Asset {
	if x != nil {
		return x.Assets
	}
	return nil
}

type DownloadHistoricalDataRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Symbols   []string `protobuf:"bytes,1,rep,name=symbols,proto3" json:"symbols,omitempty"`
	StartDate *pb.Date `protobuf:"bytes,2,opt,name=start_date,json=startDate,proto3" json:"start_date,omitempty"`
	EndDate   *pb.Date `protobuf:"bytes,3,opt,name=end_date,json=endDate,proto3" json:"end_date,omitempty"`
}

func (x *DownloadHistoricalDataRequest) Reset() {
	*x = DownloadHistoricalDataRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DownloadHistoricalDataRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DownloadHistoricalDataRequest) ProtoMessage() {}

func (x *DownloadHistoricalDataRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DownloadHistoricalDataRequest.ProtoReflect.Descriptor instead.
func (*DownloadHistoricalDataRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{4}
}

func (x *DownloadHistoricalDataRequest) GetSymbols() []string {
	if x != nil {
		return x.Symbols
	}
	return nil
}

func (x *DownloadHistoricalDataRequest) GetStartDate() *pb.Date {
	if x != nil {
		return x.StartDate
	}
	return nil
}

func (x *DownloadHistoricalDataRequest) GetEndDate() *pb.Date {
	if x != nil {
		return x.EndDate
	}
	return nil
}

type DownloadHistoricalDataResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DownloadHistoricalDataResponse) Reset() {
	*x = DownloadHistoricalDataResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DownloadHistoricalDataResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DownloadHistoricalDataResponse) ProtoMessage() {}

func (x *DownloadHistoricalDataResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_finance_marketdata_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DownloadHistoricalDataResponse.ProtoReflect.Descriptor instead.
func (*DownloadHistoricalDataResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_finance_marketdata_service_proto_rawDescGZIP(), []int{5}
}

var File_foreverbull_finance_marketdata_service_proto protoreflect.FileDescriptor

var file_foreverbull_finance_marketdata_service_proto_rawDesc = []byte{
	0x0a, 0x2c, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69,
	0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x6d, 0x61, 0x72, 0x6b, 0x65, 0x74, 0x64, 0x61, 0x74, 0x61,
	0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x13,
	0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61,
	0x6e, 0x63, 0x65, 0x1a, 0x18, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c,
	0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e,
	0x63, 0x65, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x29, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x41, 0x73, 0x73, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x22, 0x44, 0x0a, 0x10, 0x47,
	0x65, 0x74, 0x41, 0x73, 0x73, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x30, 0x0a, 0x05, 0x61, 0x73, 0x73, 0x65, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a,
	0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e,
	0x61, 0x6e, 0x63, 0x65, 0x2e, 0x41, 0x73, 0x73, 0x65, 0x74, 0x52, 0x05, 0x61, 0x73, 0x73, 0x65,
	0x74, 0x22, 0x29, 0x0a, 0x0f, 0x47, 0x65, 0x74, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x22, 0x46, 0x0a, 0x10,
	0x47, 0x65, 0x74, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x32, 0x0a, 0x06, 0x61, 0x73, 0x73, 0x65, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66,
	0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x41, 0x73, 0x73, 0x65, 0x74, 0x52, 0x06, 0x61, 0x73,
	0x73, 0x65, 0x74, 0x73, 0x22, 0xa7, 0x01, 0x0a, 0x1d, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61,
	0x64, 0x48, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x69, 0x63, 0x61, 0x6c, 0x44, 0x61, 0x74, 0x61, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x73,
	0x12, 0x37, 0x0a, 0x0a, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75,
	0x6c, 0x6c, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x44, 0x61, 0x74, 0x65, 0x52, 0x09,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x44, 0x61, 0x74, 0x65, 0x12, 0x33, 0x0a, 0x08, 0x65, 0x6e, 0x64,
	0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e,
	0x2e, 0x44, 0x61, 0x74, 0x65, 0x52, 0x07, 0x65, 0x6e, 0x64, 0x44, 0x61, 0x74, 0x65, 0x22, 0x20,
	0x0a, 0x1e, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x48, 0x69, 0x73, 0x74, 0x6f, 0x72,
	0x69, 0x63, 0x61, 0x6c, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x32, 0xc8, 0x02, 0x0a, 0x0a, 0x4d, 0x61, 0x72, 0x6b, 0x65, 0x74, 0x64, 0x61, 0x74, 0x61, 0x12,
	0x59, 0x0a, 0x08, 0x47, 0x65, 0x74, 0x41, 0x73, 0x73, 0x65, 0x74, 0x12, 0x24, 0x2e, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63,
	0x65, 0x2e, 0x47, 0x65, 0x74, 0x41, 0x73, 0x73, 0x65, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x25, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e,
	0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x41, 0x73, 0x73, 0x65, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x59, 0x0a, 0x08, 0x47, 0x65,
	0x74, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x24, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x47, 0x65, 0x74,
	0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x25, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e,
	0x63, 0x65, 0x2e, 0x47, 0x65, 0x74, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x83, 0x01, 0x0a, 0x16, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f,
	0x61, 0x64, 0x48, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x69, 0x63, 0x61, 0x6c, 0x44, 0x61, 0x74, 0x61,
	0x12, 0x32, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66,
	0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x44, 0x6f, 0x77, 0x6e, 0x6c, 0x6f, 0x61, 0x64, 0x48,
	0x69, 0x73, 0x74, 0x6f, 0x72, 0x69, 0x63, 0x61, 0x6c, 0x44, 0x61, 0x74, 0x61, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x33, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75,
	0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x44, 0x6f, 0x77, 0x6e, 0x6c,
	0x6f, 0x61, 0x64, 0x48, 0x69, 0x73, 0x74, 0x6f, 0x72, 0x69, 0x63, 0x61, 0x6c, 0x44, 0x61, 0x74,
	0x61, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x32, 0x5a, 0x30, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x68, 0x6a, 0x6e, 0x69, 0x6c,
	0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c,
	0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x70, 0x62, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_finance_marketdata_service_proto_rawDescOnce sync.Once
	file_foreverbull_finance_marketdata_service_proto_rawDescData = file_foreverbull_finance_marketdata_service_proto_rawDesc
)

func file_foreverbull_finance_marketdata_service_proto_rawDescGZIP() []byte {
	file_foreverbull_finance_marketdata_service_proto_rawDescOnce.Do(func() {
		file_foreverbull_finance_marketdata_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_finance_marketdata_service_proto_rawDescData)
	})
	return file_foreverbull_finance_marketdata_service_proto_rawDescData
}

var file_foreverbull_finance_marketdata_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_foreverbull_finance_marketdata_service_proto_goTypes = []any{
	(*GetAssetRequest)(nil),                // 0: foreverbull.finance.GetAssetRequest
	(*GetAssetResponse)(nil),               // 1: foreverbull.finance.GetAssetResponse
	(*GetIndexRequest)(nil),                // 2: foreverbull.finance.GetIndexRequest
	(*GetIndexResponse)(nil),               // 3: foreverbull.finance.GetIndexResponse
	(*DownloadHistoricalDataRequest)(nil),  // 4: foreverbull.finance.DownloadHistoricalDataRequest
	(*DownloadHistoricalDataResponse)(nil), // 5: foreverbull.finance.DownloadHistoricalDataResponse
	(*Asset)(nil),                          // 6: foreverbull.finance.Asset
	(*pb.Date)(nil),                        // 7: foreverbull.common.Date
}
var file_foreverbull_finance_marketdata_service_proto_depIdxs = []int32{
	6, // 0: foreverbull.finance.GetAssetResponse.asset:type_name -> foreverbull.finance.Asset
	6, // 1: foreverbull.finance.GetIndexResponse.assets:type_name -> foreverbull.finance.Asset
	7, // 2: foreverbull.finance.DownloadHistoricalDataRequest.start_date:type_name -> foreverbull.common.Date
	7, // 3: foreverbull.finance.DownloadHistoricalDataRequest.end_date:type_name -> foreverbull.common.Date
	0, // 4: foreverbull.finance.Marketdata.GetAsset:input_type -> foreverbull.finance.GetAssetRequest
	2, // 5: foreverbull.finance.Marketdata.GetIndex:input_type -> foreverbull.finance.GetIndexRequest
	4, // 6: foreverbull.finance.Marketdata.DownloadHistoricalData:input_type -> foreverbull.finance.DownloadHistoricalDataRequest
	1, // 7: foreverbull.finance.Marketdata.GetAsset:output_type -> foreverbull.finance.GetAssetResponse
	3, // 8: foreverbull.finance.Marketdata.GetIndex:output_type -> foreverbull.finance.GetIndexResponse
	5, // 9: foreverbull.finance.Marketdata.DownloadHistoricalData:output_type -> foreverbull.finance.DownloadHistoricalDataResponse
	7, // [7:10] is the sub-list for method output_type
	4, // [4:7] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_foreverbull_finance_marketdata_service_proto_init() }
func file_foreverbull_finance_marketdata_service_proto_init() {
	if File_foreverbull_finance_marketdata_service_proto != nil {
		return
	}
	file_foreverbull_finance_finance_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_finance_marketdata_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*GetAssetRequest); i {
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
		file_foreverbull_finance_marketdata_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*GetAssetResponse); i {
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
		file_foreverbull_finance_marketdata_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*GetIndexRequest); i {
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
		file_foreverbull_finance_marketdata_service_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*GetIndexResponse); i {
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
		file_foreverbull_finance_marketdata_service_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*DownloadHistoricalDataRequest); i {
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
		file_foreverbull_finance_marketdata_service_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*DownloadHistoricalDataResponse); i {
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
			RawDescriptor: file_foreverbull_finance_marketdata_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_foreverbull_finance_marketdata_service_proto_goTypes,
		DependencyIndexes: file_foreverbull_finance_marketdata_service_proto_depIdxs,
		MessageInfos:      file_foreverbull_finance_marketdata_service_proto_msgTypes,
	}.Build()
	File_foreverbull_finance_marketdata_service_proto = out.File
	file_foreverbull_finance_marketdata_service_proto_rawDesc = nil
	file_foreverbull_finance_marketdata_service_proto_goTypes = nil
	file_foreverbull_finance_marketdata_service_proto_depIdxs = nil
}
