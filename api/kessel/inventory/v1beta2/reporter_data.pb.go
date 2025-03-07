// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: kessel/inventory/v1beta2/reporter_data.proto

package v1beta2

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ReporterData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ReporterType       string           `protobuf:"bytes,1,opt,name=reporter_type,json=reporterType,proto3" json:"reporter_type,omitempty"`
	ReporterInstanceId string           `protobuf:"bytes,2,opt,name=reporter_instance_id,json=reporterInstanceId,proto3" json:"reporter_instance_id,omitempty"`
	LocalResourceId    string           `protobuf:"bytes,4,opt,name=local_resource_id,json=localResourceId,proto3" json:"local_resource_id,omitempty"`
	ApiHref            string           `protobuf:"bytes,5,opt,name=api_href,json=apiHref,proto3" json:"api_href,omitempty"`
	ConsoleHref        string           `protobuf:"bytes,6,opt,name=console_href,json=consoleHref,proto3" json:"console_href,omitempty"`
	ResourceData       *structpb.Struct `protobuf:"bytes,7,opt,name=resource_data,json=resourceData,proto3" json:"resource_data,omitempty"`
}

func (x *ReporterData) Reset() {
	*x = ReporterData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta2_reporter_data_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReporterData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReporterData) ProtoMessage() {}

func (x *ReporterData) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta2_reporter_data_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReporterData.ProtoReflect.Descriptor instead.
func (*ReporterData) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta2_reporter_data_proto_rawDescGZIP(), []int{0}
}

func (x *ReporterData) GetReporterType() string {
	if x != nil {
		return x.ReporterType
	}
	return ""
}

func (x *ReporterData) GetReporterInstanceId() string {
	if x != nil {
		return x.ReporterInstanceId
	}
	return ""
}

func (x *ReporterData) GetLocalResourceId() string {
	if x != nil {
		return x.LocalResourceId
	}
	return ""
}

func (x *ReporterData) GetApiHref() string {
	if x != nil {
		return x.ApiHref
	}
	return ""
}

func (x *ReporterData) GetConsoleHref() string {
	if x != nil {
		return x.ConsoleHref
	}
	return ""
}

func (x *ReporterData) GetResourceData() *structpb.Struct {
	if x != nil {
		return x.ResourceData
	}
	return nil
}

var File_kessel_inventory_v1beta2_reporter_data_proto protoreflect.FileDescriptor

var file_kessel_inventory_v1beta2_reporter_data_proto_rawDesc = []byte{
	0x0a, 0x2c, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x2f, 0x72, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x65, 0x72, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69,
	0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0xbf, 0x02, 0x0a, 0x0c, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72,
	0x44, 0x61, 0x74, 0x61, 0x12, 0x2c, 0x0a, 0x0d, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72,
	0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48, 0x04,
	0x72, 0x02, 0x10, 0x01, 0x52, 0x0c, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x39, 0x0a, 0x14, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x72, 0x5f, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x42, 0x07, 0xba, 0x48, 0x04, 0x72, 0x02, 0x10, 0x01, 0x52, 0x12, 0x72, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x65, 0x72, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x49, 0x64, 0x12, 0x33, 0x0a,
	0x11, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f,
	0x69, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48, 0x04, 0x72, 0x02, 0x10,
	0x01, 0x52, 0x0f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x49, 0x64, 0x12, 0x22, 0x0a, 0x08, 0x61, 0x70, 0x69, 0x5f, 0x68, 0x72, 0x65, 0x66, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48, 0x04, 0x72, 0x02, 0x10, 0x01, 0x52, 0x07, 0x61,
	0x70, 0x69, 0x48, 0x72, 0x65, 0x66, 0x12, 0x2a, 0x0a, 0x0c, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c,
	0x65, 0x5f, 0x68, 0x72, 0x65, 0x66, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x42, 0x07, 0xba, 0x48,
	0x04, 0x72, 0x02, 0x10, 0x01, 0x52, 0x0b, 0x63, 0x6f, 0x6e, 0x73, 0x6f, 0x6c, 0x65, 0x48, 0x72,
	0x65, 0x66, 0x12, 0x41, 0x0a, 0x0d, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74, 0x72, 0x75,
	0x63, 0x74, 0x42, 0x03, 0xba, 0x48, 0x00, 0x52, 0x0c, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x44, 0x61, 0x74, 0x61, 0x42, 0x72, 0x0a, 0x28, 0x6f, 0x72, 0x67, 0x2e, 0x70, 0x72, 0x6f,
	0x6a, 0x65, 0x63, 0x74, 0x5f, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x61, 0x70, 0x69, 0x2e,
	0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x32, 0x50, 0x01, 0x5a, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x2d, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69,
	0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2d, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x70, 0x69,
	0x2f, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72,
	0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_kessel_inventory_v1beta2_reporter_data_proto_rawDescOnce sync.Once
	file_kessel_inventory_v1beta2_reporter_data_proto_rawDescData = file_kessel_inventory_v1beta2_reporter_data_proto_rawDesc
)

func file_kessel_inventory_v1beta2_reporter_data_proto_rawDescGZIP() []byte {
	file_kessel_inventory_v1beta2_reporter_data_proto_rawDescOnce.Do(func() {
		file_kessel_inventory_v1beta2_reporter_data_proto_rawDescData = protoimpl.X.CompressGZIP(file_kessel_inventory_v1beta2_reporter_data_proto_rawDescData)
	})
	return file_kessel_inventory_v1beta2_reporter_data_proto_rawDescData
}

var file_kessel_inventory_v1beta2_reporter_data_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_kessel_inventory_v1beta2_reporter_data_proto_goTypes = []any{
	(*ReporterData)(nil),    // 0: kessel.inventory.v1beta2.ReporterData
	(*structpb.Struct)(nil), // 1: google.protobuf.Struct
}
var file_kessel_inventory_v1beta2_reporter_data_proto_depIdxs = []int32{
	1, // 0: kessel.inventory.v1beta2.ReporterData.resource_data:type_name -> google.protobuf.Struct
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_kessel_inventory_v1beta2_reporter_data_proto_init() }
func file_kessel_inventory_v1beta2_reporter_data_proto_init() {
	if File_kessel_inventory_v1beta2_reporter_data_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_kessel_inventory_v1beta2_reporter_data_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*ReporterData); i {
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
			RawDescriptor: file_kessel_inventory_v1beta2_reporter_data_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_kessel_inventory_v1beta2_reporter_data_proto_goTypes,
		DependencyIndexes: file_kessel_inventory_v1beta2_reporter_data_proto_depIdxs,
		MessageInfos:      file_kessel_inventory_v1beta2_reporter_data_proto_msgTypes,
	}.Build()
	File_kessel_inventory_v1beta2_reporter_data_proto = out.File
	file_kessel_inventory_v1beta2_reporter_data_proto_rawDesc = nil
	file_kessel_inventory_v1beta2_reporter_data_proto_goTypes = nil
	file_kessel_inventory_v1beta2_reporter_data_proto_depIdxs = nil
}
