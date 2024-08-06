// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: kessel/inventory/v1beta1/metadata.proto

package v1beta1

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
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

type Metadata struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Kessel Asset Inventory generated identifier.
	Id int64 `protobuf:"varint,3355,opt,name=id,proto3" json:"id,omitempty"`
	// Some identifier intrinsic to the resource itself that is unique across reporters
	NaturalId string `protobuf:"bytes,1,opt,name=natural_id,json=naturalId,proto3" json:"natural_id,omitempty"`
	// The type of the Resource
	ResourceType string `protobuf:"bytes,442752204,opt,name=resource_type,json=resourceType,proto3" json:"resource_type,omitempty"`
	// Date and time when the inventory item was first reported.
	FirstReported *timestamppb.Timestamp `protobuf:"bytes,13874816,opt,name=first_reported,json=firstReported,proto3" json:"first_reported,omitempty"`
	// Date and time when the inventory item was last updated.
	LastReported *timestamppb.Timestamp `protobuf:"bytes,436473483,opt,name=last_reported,json=lastReported,proto3" json:"last_reported,omitempty"`
	// Identifier of the reporter that first reported on this item.
	FirstReportedBy string `protobuf:"bytes,46112820,opt,name=first_reported_by,json=firstReportedBy,proto3" json:"first_reported_by,omitempty"`
	// Identifier of the reporter that last reported on this item.
	LastReportedBy string `protobuf:"bytes,505008782,opt,name=last_reported_by,json=lastReportedBy,proto3" json:"last_reported_by,omitempty"`
	// The workspace in which this resource is a member for access control.  A
	// resource can only be a member of one workspace.
	Workspace string           `protobuf:"bytes,35122327,opt,name=workspace,proto3" json:"workspace,omitempty"`
	Labels    []*ResourceLabel `protobuf:"bytes,3552281,rep,name=labels,proto3" json:"labels,omitempty"`
}

func (x *Metadata) Reset() {
	*x = Metadata{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_metadata_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metadata) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metadata) ProtoMessage() {}

func (x *Metadata) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_metadata_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metadata.ProtoReflect.Descriptor instead.
func (*Metadata) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_metadata_proto_rawDescGZIP(), []int{0}
}

func (x *Metadata) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Metadata) GetNaturalId() string {
	if x != nil {
		return x.NaturalId
	}
	return ""
}

func (x *Metadata) GetResourceType() string {
	if x != nil {
		return x.ResourceType
	}
	return ""
}

func (x *Metadata) GetFirstReported() *timestamppb.Timestamp {
	if x != nil {
		return x.FirstReported
	}
	return nil
}

func (x *Metadata) GetLastReported() *timestamppb.Timestamp {
	if x != nil {
		return x.LastReported
	}
	return nil
}

func (x *Metadata) GetFirstReportedBy() string {
	if x != nil {
		return x.FirstReportedBy
	}
	return ""
}

func (x *Metadata) GetLastReportedBy() string {
	if x != nil {
		return x.LastReportedBy
	}
	return ""
}

func (x *Metadata) GetWorkspace() string {
	if x != nil {
		return x.Workspace
	}
	return ""
}

func (x *Metadata) GetLabels() []*ResourceLabel {
	if x != nil {
		return x.Labels
	}
	return nil
}

var File_kessel_inventory_v1beta1_metadata_proto protoreflect.FileDescriptor

var file_kessel_inventory_v1beta1_metadata_proto_rawDesc = []byte{
	0x0a, 0x27, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x6d, 0x65, 0x74, 0x61, 0x64,
	0x61, 0x74, 0x61, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18, 0x6b, 0x65, 0x73, 0x73, 0x65,
	0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f,
	0x66, 0x69, 0x65, 0x6c, 0x64, 0x5f, 0x62, 0x65, 0x68, 0x61, 0x76, 0x69, 0x6f, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x2d, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e,
	0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f,
	0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xc9, 0x03, 0x0a, 0x08, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x14, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x9b, 0x1a, 0x20, 0x01, 0x28, 0x03, 0x42, 0x03,
	0xe0, 0x41, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x6e, 0x61, 0x74, 0x75, 0x72,
	0x61, 0x6c, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x74,
	0x75, 0x72, 0x61, 0x6c, 0x49, 0x64, 0x12, 0x27, 0x0a, 0x0d, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0xcc, 0xb9, 0x8f, 0xd3, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0c, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x49, 0x0a, 0x0e, 0x66, 0x69, 0x72, 0x73, 0x74, 0x5f, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65,
	0x64, 0x18, 0x80, 0xed, 0xce, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d,
	0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x42, 0x03, 0xe0, 0x41, 0x03, 0x52, 0x0d, 0x66, 0x69, 0x72,
	0x73, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x12, 0x48, 0x0a, 0x0d, 0x6c, 0x61,
	0x73, 0x74, 0x5f, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x18, 0x8b, 0x9d, 0x90, 0xd0,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x42, 0x03, 0xe0, 0x41, 0x03, 0x52, 0x0c, 0x6c, 0x61, 0x73, 0x74, 0x52, 0x65, 0x70, 0x6f,
	0x72, 0x74, 0x65, 0x64, 0x12, 0x32, 0x0a, 0x11, 0x66, 0x69, 0x72, 0x73, 0x74, 0x5f, 0x72, 0x65,
	0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x5f, 0x62, 0x79, 0x18, 0xb4, 0xc0, 0xfe, 0x15, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x03, 0xe0, 0x41, 0x03, 0x52, 0x0f, 0x66, 0x69, 0x72, 0x73, 0x74, 0x52, 0x65,
	0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x42, 0x79, 0x12, 0x31, 0x0a, 0x10, 0x6c, 0x61, 0x73, 0x74,
	0x5f, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x5f, 0x62, 0x79, 0x18, 0x8e, 0xa5, 0xe7,
	0xf0, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x03, 0xe0, 0x41, 0x03, 0x52, 0x0e, 0x6c, 0x61, 0x73,
	0x74, 0x52, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x42, 0x79, 0x12, 0x1f, 0x0a, 0x09, 0x77,
	0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x97, 0xd9, 0xdf, 0x10, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x42, 0x0a, 0x06,
	0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73, 0x18, 0x99, 0xe8, 0xd8, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x27, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x52, 0x06, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x73,
	0x42, 0x72, 0x0a, 0x28, 0x6f, 0x72, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x5f,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e,
	0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x50, 0x01, 0x5a, 0x44,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x6a, 0x65,
	0x63, 0x74, 0x2d, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74,
	0x6f, 0x72, 0x79, 0x2d, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6b, 0x65, 0x73, 0x73,
	0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_kessel_inventory_v1beta1_metadata_proto_rawDescOnce sync.Once
	file_kessel_inventory_v1beta1_metadata_proto_rawDescData = file_kessel_inventory_v1beta1_metadata_proto_rawDesc
)

func file_kessel_inventory_v1beta1_metadata_proto_rawDescGZIP() []byte {
	file_kessel_inventory_v1beta1_metadata_proto_rawDescOnce.Do(func() {
		file_kessel_inventory_v1beta1_metadata_proto_rawDescData = protoimpl.X.CompressGZIP(file_kessel_inventory_v1beta1_metadata_proto_rawDescData)
	})
	return file_kessel_inventory_v1beta1_metadata_proto_rawDescData
}

var file_kessel_inventory_v1beta1_metadata_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_kessel_inventory_v1beta1_metadata_proto_goTypes = []any{
	(*Metadata)(nil),              // 0: kessel.inventory.v1beta1.Metadata
	(*timestamppb.Timestamp)(nil), // 1: google.protobuf.Timestamp
	(*ResourceLabel)(nil),         // 2: kessel.inventory.v1beta1.ResourceLabel
}
var file_kessel_inventory_v1beta1_metadata_proto_depIdxs = []int32{
	1, // 0: kessel.inventory.v1beta1.Metadata.first_reported:type_name -> google.protobuf.Timestamp
	1, // 1: kessel.inventory.v1beta1.Metadata.last_reported:type_name -> google.protobuf.Timestamp
	2, // 2: kessel.inventory.v1beta1.Metadata.labels:type_name -> kessel.inventory.v1beta1.ResourceLabel
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_kessel_inventory_v1beta1_metadata_proto_init() }
func file_kessel_inventory_v1beta1_metadata_proto_init() {
	if File_kessel_inventory_v1beta1_metadata_proto != nil {
		return
	}
	file_kessel_inventory_v1beta1_resource_label_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_kessel_inventory_v1beta1_metadata_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*Metadata); i {
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
			RawDescriptor: file_kessel_inventory_v1beta1_metadata_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_kessel_inventory_v1beta1_metadata_proto_goTypes,
		DependencyIndexes: file_kessel_inventory_v1beta1_metadata_proto_depIdxs,
		MessageInfos:      file_kessel_inventory_v1beta1_metadata_proto_msgTypes,
	}.Build()
	File_kessel_inventory_v1beta1_metadata_proto = out.File
	file_kessel_inventory_v1beta1_metadata_proto_rawDesc = nil
	file_kessel_inventory_v1beta1_metadata_proto_goTypes = nil
	file_kessel_inventory_v1beta1_metadata_proto_depIdxs = nil
}
