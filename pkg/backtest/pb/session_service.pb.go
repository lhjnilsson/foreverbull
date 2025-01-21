// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/backtest/session_service.proto

package pb

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	pb1 "github.com/lhjnilsson/foreverbull/pkg/finance/pb"
	pb "github.com/lhjnilsson/foreverbull/pkg/service/pb"
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

type CreateExecutionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Backtest  *Backtest     `protobuf:"bytes,1,opt,name=backtest,proto3" json:"backtest,omitempty"`
	Algorithm *pb.Algorithm `protobuf:"bytes,2,opt,name=algorithm,proto3" json:"algorithm,omitempty"`
}

func (x *CreateExecutionRequest) Reset() {
	*x = CreateExecutionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateExecutionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateExecutionRequest) ProtoMessage() {}

func (x *CreateExecutionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateExecutionRequest.ProtoReflect.Descriptor instead.
func (*CreateExecutionRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{0}
}

func (x *CreateExecutionRequest) GetBacktest() *Backtest {
	if x != nil {
		return x.Backtest
	}
	return nil
}

func (x *CreateExecutionRequest) GetAlgorithm() *pb.Algorithm {
	if x != nil {
		return x.Algorithm
	}
	return nil
}

type CreateExecutionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Execution     *Execution                 `protobuf:"bytes,1,opt,name=execution,proto3" json:"execution,omitempty"`
	Configuration *pb.ExecutionConfiguration `protobuf:"bytes,2,opt,name=configuration,proto3" json:"configuration,omitempty"`
}

func (x *CreateExecutionResponse) Reset() {
	*x = CreateExecutionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateExecutionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateExecutionResponse) ProtoMessage() {}

func (x *CreateExecutionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateExecutionResponse.ProtoReflect.Descriptor instead.
func (*CreateExecutionResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{1}
}

func (x *CreateExecutionResponse) GetExecution() *Execution {
	if x != nil {
		return x.Execution
	}
	return nil
}

func (x *CreateExecutionResponse) GetConfiguration() *pb.ExecutionConfiguration {
	if x != nil {
		return x.Configuration
	}
	return nil
}

type RunExecutionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExecutionId string `protobuf:"bytes,1,opt,name=execution_id,json=executionId,proto3" json:"execution_id,omitempty"`
}

func (x *RunExecutionRequest) Reset() {
	*x = RunExecutionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunExecutionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunExecutionRequest) ProtoMessage() {}

func (x *RunExecutionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunExecutionRequest.ProtoReflect.Descriptor instead.
func (*RunExecutionRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{2}
}

func (x *RunExecutionRequest) GetExecutionId() string {
	if x != nil {
		return x.ExecutionId
	}
	return ""
}

type RunExecutionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Execution *Execution     `protobuf:"bytes,1,opt,name=execution,proto3" json:"execution,omitempty"`
	Portfolio *pb1.Portfolio `protobuf:"bytes,2,opt,name=portfolio,proto3" json:"portfolio,omitempty"`
}

func (x *RunExecutionResponse) Reset() {
	*x = RunExecutionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunExecutionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunExecutionResponse) ProtoMessage() {}

func (x *RunExecutionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunExecutionResponse.ProtoReflect.Descriptor instead.
func (*RunExecutionResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{3}
}

func (x *RunExecutionResponse) GetExecution() *Execution {
	if x != nil {
		return x.Execution
	}
	return nil
}

func (x *RunExecutionResponse) GetPortfolio() *pb1.Portfolio {
	if x != nil {
		return x.Portfolio
	}
	return nil
}

type StoreExecutionResultRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExecutionId string `protobuf:"bytes,1,opt,name=execution_id,json=executionId,proto3" json:"execution_id,omitempty"`
}

func (x *StoreExecutionResultRequest) Reset() {
	*x = StoreExecutionResultRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StoreExecutionResultRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StoreExecutionResultRequest) ProtoMessage() {}

func (x *StoreExecutionResultRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StoreExecutionResultRequest.ProtoReflect.Descriptor instead.
func (*StoreExecutionResultRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{4}
}

func (x *StoreExecutionResultRequest) GetExecutionId() string {
	if x != nil {
		return x.ExecutionId
	}
	return ""
}

type StoreExecutionResultResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StoreExecutionResultResponse) Reset() {
	*x = StoreExecutionResultResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StoreExecutionResultResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StoreExecutionResultResponse) ProtoMessage() {}

func (x *StoreExecutionResultResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StoreExecutionResultResponse.ProtoReflect.Descriptor instead.
func (*StoreExecutionResultResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{5}
}

type StopServerRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StopServerRequest) Reset() {
	*x = StopServerRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StopServerRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopServerRequest) ProtoMessage() {}

func (x *StopServerRequest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopServerRequest.ProtoReflect.Descriptor instead.
func (*StopServerRequest) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{6}
}

type StopServerResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StopServerResponse) Reset() {
	*x = StopServerResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_session_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StopServerResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StopServerResponse) ProtoMessage() {}

func (x *StopServerResponse) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_session_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StopServerResponse.ProtoReflect.Descriptor instead.
func (*StopServerResponse) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_session_service_proto_rawDescGZIP(), []int{7}
}

var File_foreverbull_backtest_session_service_proto protoreflect.FileDescriptor

var file_foreverbull_backtest_session_service_proto_rawDesc = []byte{
	0x0a, 0x2a, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x62, 0x61,
	0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65,
	0x73, 0x74, 0x1a, 0x23, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f,
	0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x20, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x77, 0x6f, 0x72,
	0x6b, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x66, 0x6f, 0x72, 0x65, 0x76,
	0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x66, 0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2f, 0x66,
	0x69, 0x6e, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x24, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65,
	0x73, 0x74, 0x2f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65,
	0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xe6, 0x01, 0x0a, 0x16, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x66, 0x0a, 0x08, 0x62, 0x61,
	0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x42, 0x2a, 0xba, 0x48,
	0x27, 0xba, 0x01, 0x21, 0x0a, 0x11, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x5f, 0x62,
	0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x0c, 0x74, 0x68, 0x69, 0x73, 0x20, 0x21, 0x3d,
	0x20, 0x6e, 0x75, 0x6c, 0x6c, 0xc8, 0x01, 0x01, 0x52, 0x08, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65,
	0x73, 0x74, 0x12, 0x64, 0x0a, 0x09, 0x61, 0x6c, 0x67, 0x6f, 0x72, 0x69, 0x74, 0x68, 0x6d, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62,
	0x75, 0x6c, 0x6c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x41, 0x6c, 0x67, 0x6f,
	0x72, 0x69, 0x74, 0x68, 0x6d, 0x42, 0x26, 0xba, 0x48, 0x23, 0xba, 0x01, 0x1d, 0x0a, 0x0d, 0x72,
	0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x5f, 0x61, 0x6c, 0x67, 0x6f, 0x1a, 0x0c, 0x74, 0x68,
	0x69, 0x73, 0x20, 0x21, 0x3d, 0x20, 0x6e, 0x75, 0x6c, 0x6c, 0xc8, 0x01, 0x01, 0x52, 0x09, 0x61,
	0x6c, 0x67, 0x6f, 0x72, 0x69, 0x74, 0x68, 0x6d, 0x22, 0xab, 0x01, 0x0a, 0x17, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3d, 0x0a, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65,
	0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x45,
	0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74,
	0x69, 0x6f, 0x6e, 0x12, 0x51, 0x0a, 0x0d, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2b, 0x2e, 0x66, 0x6f, 0x72,
	0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x2e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x59, 0x0a, 0x13, 0x52, 0x75, 0x6e, 0x45, 0x78, 0x65,
	0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x42, 0x0a,
	0x0c, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x42, 0x1f, 0xba, 0x48, 0x1c, 0xba, 0x01, 0x16, 0x0a, 0x08, 0x72, 0x65, 0x71,
	0x75, 0x69, 0x72, 0x65, 0x64, 0x1a, 0x0a, 0x74, 0x68, 0x69, 0x73, 0x20, 0x21, 0x3d, 0x20, 0x27,
	0x27, 0xc8, 0x01, 0x01, 0x52, 0x0b, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x49,
	0x64, 0x22, 0x93, 0x01, 0x0a, 0x14, 0x52, 0x75, 0x6e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x3d, 0x0a, 0x09, 0x65, 0x78,
	0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e,
	0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b,
	0x74, 0x65, 0x73, 0x74, 0x2e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09,
	0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x3c, 0x0a, 0x09, 0x70, 0x6f, 0x72,
	0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x66, 0x69, 0x6e, 0x61, 0x6e,
	0x63, 0x65, 0x2e, 0x50, 0x6f, 0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x52, 0x09, 0x70, 0x6f,
	0x72, 0x74, 0x66, 0x6f, 0x6c, 0x69, 0x6f, 0x22, 0x61, 0x0a, 0x1b, 0x53, 0x74, 0x6f, 0x72, 0x65,
	0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x42, 0x0a, 0x0c, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74,
	0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x1f, 0xba, 0x48,
	0x1c, 0xba, 0x01, 0x16, 0x0a, 0x08, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x1a, 0x0a,
	0x74, 0x68, 0x69, 0x73, 0x20, 0x21, 0x3d, 0x20, 0x27, 0x27, 0xc8, 0x01, 0x01, 0x52, 0x0b, 0x65,
	0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x64, 0x22, 0x1e, 0x0a, 0x1c, 0x53, 0x74,
	0x6f, 0x72, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x75,
	0x6c, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x13, 0x0a, 0x11, 0x53, 0x74,
	0x6f, 0x70, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22,
	0x14, 0x0a, 0x12, 0x53, 0x74, 0x6f, 0x70, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xc9, 0x03, 0x0a, 0x0f, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x72, 0x12, 0x70, 0x0a, 0x0f, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x2c, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2d, 0x2e, 0x66, 0x6f, 0x72,
	0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x69, 0x0a, 0x0c, 0x52,
	0x75, 0x6e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x29, 0x2e, 0x66, 0x6f,
	0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65,
	0x73, 0x74, 0x2e, 0x52, 0x75, 0x6e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2a, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x52, 0x75,
	0x6e, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x30, 0x01, 0x12, 0x76, 0x0a, 0x0b, 0x53, 0x74, 0x6f, 0x72, 0x65, 0x52,
	0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x31, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62,
	0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x6f,
	0x72, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x32, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76,
	0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e,
	0x53, 0x74, 0x6f, 0x72, 0x65, 0x45, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65,
	0x73, 0x75, 0x6c, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x61,
	0x0a, 0x0a, 0x53, 0x74, 0x6f, 0x70, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12, 0x27, 0x2e, 0x66,
	0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74,
	0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x6f, 0x70, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x28, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62,
	0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x6f,
	0x70, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x42, 0x33, 0x5a, 0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x6c, 0x68, 0x6a, 0x6e, 0x69, 0x6c, 0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76,
	0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74,
	0x65, 0x73, 0x74, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_backtest_session_service_proto_rawDescOnce sync.Once
	file_foreverbull_backtest_session_service_proto_rawDescData = file_foreverbull_backtest_session_service_proto_rawDesc
)

func file_foreverbull_backtest_session_service_proto_rawDescGZIP() []byte {
	file_foreverbull_backtest_session_service_proto_rawDescOnce.Do(func() {
		file_foreverbull_backtest_session_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_backtest_session_service_proto_rawDescData)
	})
	return file_foreverbull_backtest_session_service_proto_rawDescData
}

var file_foreverbull_backtest_session_service_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_foreverbull_backtest_session_service_proto_goTypes = []any{
	(*CreateExecutionRequest)(nil),       // 0: foreverbull.backtest.CreateExecutionRequest
	(*CreateExecutionResponse)(nil),      // 1: foreverbull.backtest.CreateExecutionResponse
	(*RunExecutionRequest)(nil),          // 2: foreverbull.backtest.RunExecutionRequest
	(*RunExecutionResponse)(nil),         // 3: foreverbull.backtest.RunExecutionResponse
	(*StoreExecutionResultRequest)(nil),  // 4: foreverbull.backtest.StoreExecutionResultRequest
	(*StoreExecutionResultResponse)(nil), // 5: foreverbull.backtest.StoreExecutionResultResponse
	(*StopServerRequest)(nil),            // 6: foreverbull.backtest.StopServerRequest
	(*StopServerResponse)(nil),           // 7: foreverbull.backtest.StopServerResponse
	(*Backtest)(nil),                     // 8: foreverbull.backtest.Backtest
	(*pb.Algorithm)(nil),                 // 9: foreverbull.service.Algorithm
	(*Execution)(nil),                    // 10: foreverbull.backtest.Execution
	(*pb.ExecutionConfiguration)(nil),    // 11: foreverbull.service.ExecutionConfiguration
	(*pb1.Portfolio)(nil),                // 12: foreverbull.finance.Portfolio
}
var file_foreverbull_backtest_session_service_proto_depIdxs = []int32{
	8,  // 0: foreverbull.backtest.CreateExecutionRequest.backtest:type_name -> foreverbull.backtest.Backtest
	9,  // 1: foreverbull.backtest.CreateExecutionRequest.algorithm:type_name -> foreverbull.service.Algorithm
	10, // 2: foreverbull.backtest.CreateExecutionResponse.execution:type_name -> foreverbull.backtest.Execution
	11, // 3: foreverbull.backtest.CreateExecutionResponse.configuration:type_name -> foreverbull.service.ExecutionConfiguration
	10, // 4: foreverbull.backtest.RunExecutionResponse.execution:type_name -> foreverbull.backtest.Execution
	12, // 5: foreverbull.backtest.RunExecutionResponse.portfolio:type_name -> foreverbull.finance.Portfolio
	0,  // 6: foreverbull.backtest.SessionServicer.CreateExecution:input_type -> foreverbull.backtest.CreateExecutionRequest
	2,  // 7: foreverbull.backtest.SessionServicer.RunExecution:input_type -> foreverbull.backtest.RunExecutionRequest
	4,  // 8: foreverbull.backtest.SessionServicer.StoreResult:input_type -> foreverbull.backtest.StoreExecutionResultRequest
	6,  // 9: foreverbull.backtest.SessionServicer.StopServer:input_type -> foreverbull.backtest.StopServerRequest
	1,  // 10: foreverbull.backtest.SessionServicer.CreateExecution:output_type -> foreverbull.backtest.CreateExecutionResponse
	3,  // 11: foreverbull.backtest.SessionServicer.RunExecution:output_type -> foreverbull.backtest.RunExecutionResponse
	5,  // 12: foreverbull.backtest.SessionServicer.StoreResult:output_type -> foreverbull.backtest.StoreExecutionResultResponse
	7,  // 13: foreverbull.backtest.SessionServicer.StopServer:output_type -> foreverbull.backtest.StopServerResponse
	10, // [10:14] is the sub-list for method output_type
	6,  // [6:10] is the sub-list for method input_type
	6,  // [6:6] is the sub-list for extension type_name
	6,  // [6:6] is the sub-list for extension extendee
	0,  // [0:6] is the sub-list for field type_name
}

func init() { file_foreverbull_backtest_session_service_proto_init() }
func file_foreverbull_backtest_session_service_proto_init() {
	if File_foreverbull_backtest_session_service_proto != nil {
		return
	}
	file_foreverbull_backtest_backtest_proto_init()
	file_foreverbull_backtest_execution_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_backtest_session_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*CreateExecutionRequest); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*CreateExecutionResponse); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*RunExecutionRequest); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*RunExecutionResponse); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*StoreExecutionResultRequest); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*StoreExecutionResultResponse); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[6].Exporter = func(v any, i int) any {
			switch v := v.(*StopServerRequest); i {
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
		file_foreverbull_backtest_session_service_proto_msgTypes[7].Exporter = func(v any, i int) any {
			switch v := v.(*StopServerResponse); i {
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
			RawDescriptor: file_foreverbull_backtest_session_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_foreverbull_backtest_session_service_proto_goTypes,
		DependencyIndexes: file_foreverbull_backtest_session_service_proto_depIdxs,
		MessageInfos:      file_foreverbull_backtest_session_service_proto_msgTypes,
	}.Build()
	File_foreverbull_backtest_session_service_proto = out.File
	file_foreverbull_backtest_session_service_proto_rawDesc = nil
	file_foreverbull_backtest_session_service_proto_goTypes = nil
	file_foreverbull_backtest_session_service_proto_depIdxs = nil
}
