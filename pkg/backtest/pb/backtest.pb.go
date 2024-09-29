// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/backtest/backtest.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Backtest_Status_Status int32

const (
	Backtest_Status_CREATED Backtest_Status_Status = 0
	Backtest_Status_READY   Backtest_Status_Status = 1
	Backtest_Status_ERROR   Backtest_Status_Status = 2
)

// Enum value maps for Backtest_Status_Status.
var (
	Backtest_Status_Status_name = map[int32]string{
		0: "CREATED",
		1: "READY",
		2: "ERROR",
	}
	Backtest_Status_Status_value = map[string]int32{
		"CREATED": 0,
		"READY":   1,
		"ERROR":   2,
	}
)

func (x Backtest_Status_Status) Enum() *Backtest_Status_Status {
	p := new(Backtest_Status_Status)
	*p = x
	return p
}

func (x Backtest_Status_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Backtest_Status_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_foreverbull_backtest_backtest_proto_enumTypes[0].Descriptor()
}

func (Backtest_Status_Status) Type() protoreflect.EnumType {
	return &file_foreverbull_backtest_backtest_proto_enumTypes[0]
}

func (x Backtest_Status_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Backtest_Status_Status.Descriptor instead.
func (Backtest_Status_Status) EnumDescriptor() ([]byte, []int) {
	return file_foreverbull_backtest_backtest_proto_rawDescGZIP(), []int{0, 0, 0}
}

type Backtest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name      string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	StartDate *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=start_date,json=startDate,proto3" json:"start_date,omitempty"`
	EndDate   *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=end_date,json=endDate,proto3" json:"end_date,omitempty"`
	Symbols   []string               `protobuf:"bytes,4,rep,name=symbols,proto3" json:"symbols,omitempty"`
	Benchmark *string                `protobuf:"bytes,5,opt,name=benchmark,proto3,oneof" json:"benchmark,omitempty"`
	Statuses  []*Backtest_Status     `protobuf:"bytes,6,rep,name=statuses,proto3" json:"statuses,omitempty"`
}

func (x *Backtest) Reset() {
	*x = Backtest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_backtest_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Backtest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Backtest) ProtoMessage() {}

func (x *Backtest) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_backtest_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Backtest.ProtoReflect.Descriptor instead.
func (*Backtest) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_backtest_proto_rawDescGZIP(), []int{0}
}

func (x *Backtest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Backtest) GetStartDate() *timestamppb.Timestamp {
	if x != nil {
		return x.StartDate
	}
	return nil
}

func (x *Backtest) GetEndDate() *timestamppb.Timestamp {
	if x != nil {
		return x.EndDate
	}
	return nil
}

func (x *Backtest) GetSymbols() []string {
	if x != nil {
		return x.Symbols
	}
	return nil
}

func (x *Backtest) GetBenchmark() string {
	if x != nil && x.Benchmark != nil {
		return *x.Benchmark
	}
	return ""
}

func (x *Backtest) GetStatuses() []*Backtest_Status {
	if x != nil {
		return x.Statuses
	}
	return nil
}

type Backtest_Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status     Backtest_Status_Status `protobuf:"varint,1,opt,name=status,proto3,enum=foreverbull.backtest.Backtest_Status_Status" json:"status,omitempty"`
	Error      *string                `protobuf:"bytes,2,opt,name=error,proto3,oneof" json:"error,omitempty"`
	OccurredAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=occurred_at,json=occurredAt,proto3" json:"occurred_at,omitempty"`
}

func (x *Backtest_Status) Reset() {
	*x = Backtest_Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_backtest_backtest_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Backtest_Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Backtest_Status) ProtoMessage() {}

func (x *Backtest_Status) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_backtest_backtest_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Backtest_Status.ProtoReflect.Descriptor instead.
func (*Backtest_Status) Descriptor() ([]byte, []int) {
	return file_foreverbull_backtest_backtest_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Backtest_Status) GetStatus() Backtest_Status_Status {
	if x != nil {
		return x.Status
	}
	return Backtest_Status_CREATED
}

func (x *Backtest_Status) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

func (x *Backtest_Status) GetOccurredAt() *timestamppb.Timestamp {
	if x != nil {
		return x.OccurredAt
	}
	return nil
}

var File_foreverbull_backtest_backtest_proto protoreflect.FileDescriptor

var file_foreverbull_backtest_backtest_proto_rawDesc = []byte{
	0x0a, 0x23, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x62, 0x61,
	0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75,
	0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x1a, 0x1f, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xfe, 0x03, 0x0a,
	0x08, 0x42, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x39, 0x0a,
	0x0a, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x09, 0x73,
	0x74, 0x61, 0x72, 0x74, 0x44, 0x61, 0x74, 0x65, 0x12, 0x35, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x5f,
	0x64, 0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x07, 0x65, 0x6e, 0x64, 0x44, 0x61, 0x74, 0x65, 0x12,
	0x18, 0x0a, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x07, 0x73, 0x79, 0x6d, 0x62, 0x6f, 0x6c, 0x73, 0x12, 0x21, 0x0a, 0x09, 0x62, 0x65, 0x6e,
	0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x09,
	0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x88, 0x01, 0x01, 0x12, 0x41, 0x0a, 0x08,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x65, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x25,
	0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63,
	0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53,
	0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x08, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x65, 0x73, 0x1a,
	0xdd, 0x01, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x44, 0x0a, 0x06, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x2c, 0x2e, 0x66, 0x6f, 0x72,
	0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73,
	0x74, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x12, 0x19, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48,
	0x00, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x88, 0x01, 0x01, 0x12, 0x3b, 0x0a, 0x0b, 0x6f,
	0x63, 0x63, 0x75, 0x72, 0x72, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0a, 0x6f, 0x63,
	0x63, 0x75, 0x72, 0x72, 0x65, 0x64, 0x41, 0x74, 0x22, 0x2b, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x12, 0x0b, 0x0a, 0x07, 0x43, 0x52, 0x45, 0x41, 0x54, 0x45, 0x44, 0x10, 0x00, 0x12,
	0x09, 0x0a, 0x05, 0x52, 0x45, 0x41, 0x44, 0x59, 0x10, 0x01, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52,
	0x52, 0x4f, 0x52, 0x10, 0x02, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x42,
	0x0c, 0x0a, 0x0a, 0x5f, 0x62, 0x65, 0x6e, 0x63, 0x68, 0x6d, 0x61, 0x72, 0x6b, 0x42, 0x33, 0x5a,
	0x31, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x68, 0x6a, 0x6e,
	0x69, 0x6c, 0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75,
	0x6c, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x74, 0x65, 0x73, 0x74, 0x2f,
	0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_backtest_backtest_proto_rawDescOnce sync.Once
	file_foreverbull_backtest_backtest_proto_rawDescData = file_foreverbull_backtest_backtest_proto_rawDesc
)

func file_foreverbull_backtest_backtest_proto_rawDescGZIP() []byte {
	file_foreverbull_backtest_backtest_proto_rawDescOnce.Do(func() {
		file_foreverbull_backtest_backtest_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_backtest_backtest_proto_rawDescData)
	})
	return file_foreverbull_backtest_backtest_proto_rawDescData
}

var file_foreverbull_backtest_backtest_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_foreverbull_backtest_backtest_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_foreverbull_backtest_backtest_proto_goTypes = []any{
	(Backtest_Status_Status)(0),   // 0: foreverbull.backtest.Backtest.Status.Status
	(*Backtest)(nil),              // 1: foreverbull.backtest.Backtest
	(*Backtest_Status)(nil),       // 2: foreverbull.backtest.Backtest.Status
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
}
var file_foreverbull_backtest_backtest_proto_depIdxs = []int32{
	3, // 0: foreverbull.backtest.Backtest.start_date:type_name -> google.protobuf.Timestamp
	3, // 1: foreverbull.backtest.Backtest.end_date:type_name -> google.protobuf.Timestamp
	2, // 2: foreverbull.backtest.Backtest.statuses:type_name -> foreverbull.backtest.Backtest.Status
	0, // 3: foreverbull.backtest.Backtest.Status.status:type_name -> foreverbull.backtest.Backtest.Status.Status
	3, // 4: foreverbull.backtest.Backtest.Status.occurred_at:type_name -> google.protobuf.Timestamp
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_foreverbull_backtest_backtest_proto_init() }
func file_foreverbull_backtest_backtest_proto_init() {
	if File_foreverbull_backtest_backtest_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_backtest_backtest_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Backtest); i {
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
		file_foreverbull_backtest_backtest_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Backtest_Status); i {
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
	file_foreverbull_backtest_backtest_proto_msgTypes[0].OneofWrappers = []any{}
	file_foreverbull_backtest_backtest_proto_msgTypes[1].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_foreverbull_backtest_backtest_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_foreverbull_backtest_backtest_proto_goTypes,
		DependencyIndexes: file_foreverbull_backtest_backtest_proto_depIdxs,
		EnumInfos:         file_foreverbull_backtest_backtest_proto_enumTypes,
		MessageInfos:      file_foreverbull_backtest_backtest_proto_msgTypes,
	}.Build()
	File_foreverbull_backtest_backtest_proto = out.File
	file_foreverbull_backtest_backtest_proto_rawDesc = nil
	file_foreverbull_backtest_backtest_proto_goTypes = nil
	file_foreverbull_backtest_backtest_proto_depIdxs = nil
}