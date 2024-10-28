// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.1
// source: foreverbull/service/instance.proto

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

type Instance_Status_Status int32

const (
	Instance_Status_CREATED    Instance_Status_Status = 0
	Instance_Status_RUNNING    Instance_Status_Status = 1
	Instance_Status_CONFIGURED Instance_Status_Status = 2
	Instance_Status_EXECUTING  Instance_Status_Status = 3
	Instance_Status_COMPLETED  Instance_Status_Status = 4
	Instance_Status_ERROR      Instance_Status_Status = 5
)

// Enum value maps for Instance_Status_Status.
var (
	Instance_Status_Status_name = map[int32]string{
		0: "CREATED",
		1: "RUNNING",
		2: "CONFIGURED",
		3: "EXECUTING",
		4: "COMPLETED",
		5: "ERROR",
	}
	Instance_Status_Status_value = map[string]int32{
		"CREATED":    0,
		"RUNNING":    1,
		"CONFIGURED": 2,
		"EXECUTING":  3,
		"COMPLETED":  4,
		"ERROR":      5,
	}
)

func (x Instance_Status_Status) Enum() *Instance_Status_Status {
	p := new(Instance_Status_Status)
	*p = x
	return p
}

func (x Instance_Status_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Instance_Status_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_foreverbull_service_instance_proto_enumTypes[0].Descriptor()
}

func (Instance_Status_Status) Type() protoreflect.EnumType {
	return &file_foreverbull_service_instance_proto_enumTypes[0]
}

func (x Instance_Status_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Instance_Status_Status.Descriptor instead.
func (Instance_Status_Status) EnumDescriptor() ([]byte, []int) {
	return file_foreverbull_service_instance_proto_rawDescGZIP(), []int{0, 0, 0}
}

type Instance struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID       string             `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Image    *string            `protobuf:"bytes,2,opt,name=Image,proto3,oneof" json:"Image,omitempty"`
	Host     *string            `protobuf:"bytes,3,opt,name=Host,proto3,oneof" json:"Host,omitempty"`
	Port     *int32             `protobuf:"varint,4,opt,name=Port,proto3,oneof" json:"Port,omitempty"`
	Statuses []*Instance_Status `protobuf:"bytes,5,rep,name=statuses,proto3" json:"statuses,omitempty"`
}

func (x *Instance) Reset() {
	*x = Instance{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_service_instance_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Instance) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Instance) ProtoMessage() {}

func (x *Instance) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_service_instance_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Instance.ProtoReflect.Descriptor instead.
func (*Instance) Descriptor() ([]byte, []int) {
	return file_foreverbull_service_instance_proto_rawDescGZIP(), []int{0}
}

func (x *Instance) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *Instance) GetImage() string {
	if x != nil && x.Image != nil {
		return *x.Image
	}
	return ""
}

func (x *Instance) GetHost() string {
	if x != nil && x.Host != nil {
		return *x.Host
	}
	return ""
}

func (x *Instance) GetPort() int32 {
	if x != nil && x.Port != nil {
		return *x.Port
	}
	return 0
}

func (x *Instance) GetStatuses() []*Instance_Status {
	if x != nil {
		return x.Statuses
	}
	return nil
}

type Instance_Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status     Instance_Status_Status `protobuf:"varint,1,opt,name=status,proto3,enum=foreverbull.service.Instance_Status_Status" json:"status,omitempty"`
	Error      *string                `protobuf:"bytes,2,opt,name=error,proto3,oneof" json:"error,omitempty"`
	OccurredAt *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=OccurredAt,proto3" json:"OccurredAt,omitempty"`
}

func (x *Instance_Status) Reset() {
	*x = Instance_Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_foreverbull_service_instance_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Instance_Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Instance_Status) ProtoMessage() {}

func (x *Instance_Status) ProtoReflect() protoreflect.Message {
	mi := &file_foreverbull_service_instance_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Instance_Status.ProtoReflect.Descriptor instead.
func (*Instance_Status) Descriptor() ([]byte, []int) {
	return file_foreverbull_service_instance_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Instance_Status) GetStatus() Instance_Status_Status {
	if x != nil {
		return x.Status
	}
	return Instance_Status_CREATED
}

func (x *Instance_Status) GetError() string {
	if x != nil && x.Error != nil {
		return *x.Error
	}
	return ""
}

func (x *Instance_Status) GetOccurredAt() *timestamppb.Timestamp {
	if x != nil {
		return x.OccurredAt
	}
	return nil
}

var File_foreverbull_service_instance_proto protoreflect.FileDescriptor

var file_foreverbull_service_instance_proto_rawDesc = []byte{
	0x0a, 0x22, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x13, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c,
	0x6c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73,
	0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xd3, 0x03, 0x0a, 0x08, 0x49,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x49, 0x44, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x49, 0x44, 0x12, 0x19, 0x0a, 0x05, 0x49, 0x6d, 0x61, 0x67, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x05, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x88,
	0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x48, 0x01, 0x52, 0x04, 0x48, 0x6f, 0x73, 0x74, 0x88, 0x01, 0x01, 0x12, 0x17, 0x0a, 0x04, 0x50,
	0x6f, 0x72, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x05, 0x48, 0x02, 0x52, 0x04, 0x50, 0x6f, 0x72,
	0x74, 0x88, 0x01, 0x01, 0x12, 0x40, 0x0a, 0x08, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x65, 0x73,
	0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72,
	0x62, 0x75, 0x6c, 0x6c, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x49, 0x6e, 0x73,
	0x74, 0x61, 0x6e, 0x63, 0x65, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x08, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x65, 0x73, 0x1a, 0x8b, 0x02, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x12, 0x43, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x2b, 0x2e, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65, 0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2e,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65,
	0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x19, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x88, 0x01,
	0x01, 0x12, 0x3a, 0x0a, 0x0a, 0x4f, 0x63, 0x63, 0x75, 0x72, 0x72, 0x65, 0x64, 0x41, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x52, 0x0a, 0x4f, 0x63, 0x63, 0x75, 0x72, 0x72, 0x65, 0x64, 0x41, 0x74, 0x22, 0x5b, 0x0a,
	0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x0b, 0x0a, 0x07, 0x43, 0x52, 0x45, 0x41, 0x54,
	0x45, 0x44, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x52, 0x55, 0x4e, 0x4e, 0x49, 0x4e, 0x47, 0x10,
	0x01, 0x12, 0x0e, 0x0a, 0x0a, 0x43, 0x4f, 0x4e, 0x46, 0x49, 0x47, 0x55, 0x52, 0x45, 0x44, 0x10,
	0x02, 0x12, 0x0d, 0x0a, 0x09, 0x45, 0x58, 0x45, 0x43, 0x55, 0x54, 0x49, 0x4e, 0x47, 0x10, 0x03,
	0x12, 0x0d, 0x0a, 0x09, 0x43, 0x4f, 0x4d, 0x50, 0x4c, 0x45, 0x54, 0x45, 0x44, 0x10, 0x04, 0x12,
	0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x05, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x42, 0x08, 0x0a, 0x06, 0x5f, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x42, 0x07,
	0x0a, 0x05, 0x5f, 0x48, 0x6f, 0x73, 0x74, 0x42, 0x07, 0x0a, 0x05, 0x5f, 0x50, 0x6f, 0x72, 0x74,
	0x42, 0x32, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c,
	0x68, 0x6a, 0x6e, 0x69, 0x6c, 0x73, 0x73, 0x6f, 0x6e, 0x2f, 0x66, 0x6f, 0x72, 0x65, 0x76, 0x65,
	0x72, 0x62, 0x75, 0x6c, 0x6c, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_foreverbull_service_instance_proto_rawDescOnce sync.Once
	file_foreverbull_service_instance_proto_rawDescData = file_foreverbull_service_instance_proto_rawDesc
)

func file_foreverbull_service_instance_proto_rawDescGZIP() []byte {
	file_foreverbull_service_instance_proto_rawDescOnce.Do(func() {
		file_foreverbull_service_instance_proto_rawDescData = protoimpl.X.CompressGZIP(file_foreverbull_service_instance_proto_rawDescData)
	})
	return file_foreverbull_service_instance_proto_rawDescData
}

var file_foreverbull_service_instance_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_foreverbull_service_instance_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_foreverbull_service_instance_proto_goTypes = []any{
	(Instance_Status_Status)(0),   // 0: foreverbull.service.Instance.Status.Status
	(*Instance)(nil),              // 1: foreverbull.service.Instance
	(*Instance_Status)(nil),       // 2: foreverbull.service.Instance.Status
	(*timestamppb.Timestamp)(nil), // 3: google.protobuf.Timestamp
}
var file_foreverbull_service_instance_proto_depIdxs = []int32{
	2, // 0: foreverbull.service.Instance.statuses:type_name -> foreverbull.service.Instance.Status
	0, // 1: foreverbull.service.Instance.Status.status:type_name -> foreverbull.service.Instance.Status.Status
	3, // 2: foreverbull.service.Instance.Status.OccurredAt:type_name -> google.protobuf.Timestamp
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_foreverbull_service_instance_proto_init() }
func file_foreverbull_service_instance_proto_init() {
	if File_foreverbull_service_instance_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_foreverbull_service_instance_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Instance); i {
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
		file_foreverbull_service_instance_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*Instance_Status); i {
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
	file_foreverbull_service_instance_proto_msgTypes[0].OneofWrappers = []any{}
	file_foreverbull_service_instance_proto_msgTypes[1].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_foreverbull_service_instance_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_foreverbull_service_instance_proto_goTypes,
		DependencyIndexes: file_foreverbull_service_instance_proto_depIdxs,
		EnumInfos:         file_foreverbull_service_instance_proto_enumTypes,
		MessageInfos:      file_foreverbull_service_instance_proto_msgTypes,
	}.Build()
	File_foreverbull_service_instance_proto = out.File
	file_foreverbull_service_instance_proto_rawDesc = nil
	file_foreverbull_service_instance_proto_goTypes = nil
	file_foreverbull_service_instance_proto_depIdxs = nil
}
