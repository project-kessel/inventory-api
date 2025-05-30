// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: kessel/inventory/v1beta2/streamed_list_objects_request.proto

package v1beta2

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

type StreamedListObjectsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ObjectType  *RepresentationType `protobuf:"bytes,1,opt,name=object_type,json=objectType,proto3" json:"object_type,omitempty"`
	Relation    string              `protobuf:"bytes,2,opt,name=relation,proto3" json:"relation,omitempty"`
	Subject     *SubjectReference   `protobuf:"bytes,3,opt,name=subject,proto3" json:"subject,omitempty"`
	Pagination  *RequestPagination  `protobuf:"bytes,4,opt,name=pagination,proto3,oneof" json:"pagination,omitempty"`
	Consistency *Consistency        `protobuf:"bytes,5,opt,name=consistency,proto3,oneof" json:"consistency,omitempty"`
}

func (x *StreamedListObjectsRequest) Reset() {
	*x = StreamedListObjectsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StreamedListObjectsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StreamedListObjectsRequest) ProtoMessage() {}

func (x *StreamedListObjectsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StreamedListObjectsRequest.ProtoReflect.Descriptor instead.
func (*StreamedListObjectsRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescGZIP(), []int{0}
}

func (x *StreamedListObjectsRequest) GetObjectType() *RepresentationType {
	if x != nil {
		return x.ObjectType
	}
	return nil
}

func (x *StreamedListObjectsRequest) GetRelation() string {
	if x != nil {
		return x.Relation
	}
	return ""
}

func (x *StreamedListObjectsRequest) GetSubject() *SubjectReference {
	if x != nil {
		return x.Subject
	}
	return nil
}

func (x *StreamedListObjectsRequest) GetPagination() *RequestPagination {
	if x != nil {
		return x.Pagination
	}
	return nil
}

func (x *StreamedListObjectsRequest) GetConsistency() *Consistency {
	if x != nil {
		return x.Consistency
	}
	return nil
}

var File_kessel_inventory_v1beta2_streamed_list_objects_request_proto protoreflect.FileDescriptor

var file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDesc = []byte{
	0x0a, 0x3c, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x2f, 0x73, 0x74, 0x72, 0x65, 0x61,
	0x6d, 0x65, 0x64, 0x5f, 0x6c, 0x69, 0x73, 0x74, 0x5f, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x73,
	0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x18,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x31, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e,
	0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x2f,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x30, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c,
	0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x32, 0x2f, 0x73, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x5f, 0x72, 0x65, 0x66, 0x65, 0x72,
	0x65, 0x6e, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x2a, 0x6b, 0x65, 0x73, 0x73,
	0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x32, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x63, 0x79,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x32, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69,
	0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32,
	0x2f, 0x72, 0x65, 0x70, 0x72, 0x65, 0x73, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f,
	0x74, 0x79, 0x70, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xa5, 0x03, 0x0a, 0x1a, 0x53,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x65, 0x64, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x62, 0x6a, 0x65, 0x63,
	0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x55, 0x0a, 0x0b, 0x6f, 0x62, 0x6a,
	0x65, 0x63, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c,
	0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72,
	0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x2e, 0x52, 0x65, 0x70, 0x72, 0x65, 0x73,
	0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x42, 0x06, 0xba, 0x48,
	0x03, 0xc8, 0x01, 0x01, 0x52, 0x0a, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x54, 0x79, 0x70, 0x65,
	0x12, 0x23, 0x0a, 0x08, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x42, 0x07, 0xba, 0x48, 0x04, 0x72, 0x02, 0x10, 0x01, 0x52, 0x08, 0x72, 0x65, 0x6c,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x4c, 0x0a, 0x07, 0x73, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e,
	0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x32, 0x2e, 0x53, 0x75, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x65, 0x66, 0x65, 0x72, 0x65, 0x6e,
	0x63, 0x65, 0x42, 0x06, 0xba, 0x48, 0x03, 0xc8, 0x01, 0x01, 0x52, 0x07, 0x73, 0x75, 0x62, 0x6a,
	0x65, 0x63, 0x74, 0x12, 0x50, 0x0a, 0x0a, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2b, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c,
	0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x32, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x50, 0x61, 0x67, 0x69, 0x6e, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x48, 0x00, 0x52, 0x0a, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x88, 0x01, 0x01, 0x12, 0x4c, 0x0a, 0x0b, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74,
	0x65, 0x6e, 0x63, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x25, 0x2e, 0x6b, 0x65, 0x73,
	0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x32, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x63,
	0x79, 0x48, 0x01, 0x52, 0x0b, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x63, 0x79,
	0x88, 0x01, 0x01, 0x42, 0x0d, 0x0a, 0x0b, 0x5f, 0x70, 0x61, 0x67, 0x69, 0x6e, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x42, 0x0e, 0x0a, 0x0c, 0x5f, 0x63, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e,
	0x63, 0x79, 0x42, 0x72, 0x0a, 0x28, 0x6f, 0x72, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63,
	0x74, 0x5f, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x69, 0x6e, 0x76,
	0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x50, 0x01,
	0x5a, 0x44, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x6f,
	0x6a, 0x65, 0x63, 0x74, 0x2d, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65,
	0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2d, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6b, 0x65,
	0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x32, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescOnce sync.Once
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescData = file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDesc
)

func file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescGZIP() []byte {
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescOnce.Do(func() {
		file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescData = protoimpl.X.CompressGZIP(file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescData)
	})
	return file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDescData
}

var file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_goTypes = []any{
	(*StreamedListObjectsRequest)(nil), // 0: kessel.inventory.v1beta2.StreamedListObjectsRequest
	(*RepresentationType)(nil),         // 1: kessel.inventory.v1beta2.RepresentationType
	(*SubjectReference)(nil),           // 2: kessel.inventory.v1beta2.SubjectReference
	(*RequestPagination)(nil),          // 3: kessel.inventory.v1beta2.RequestPagination
	(*Consistency)(nil),                // 4: kessel.inventory.v1beta2.Consistency
}
var file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_depIdxs = []int32{
	1, // 0: kessel.inventory.v1beta2.StreamedListObjectsRequest.object_type:type_name -> kessel.inventory.v1beta2.RepresentationType
	2, // 1: kessel.inventory.v1beta2.StreamedListObjectsRequest.subject:type_name -> kessel.inventory.v1beta2.SubjectReference
	3, // 2: kessel.inventory.v1beta2.StreamedListObjectsRequest.pagination:type_name -> kessel.inventory.v1beta2.RequestPagination
	4, // 3: kessel.inventory.v1beta2.StreamedListObjectsRequest.consistency:type_name -> kessel.inventory.v1beta2.Consistency
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_init() }
func file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_init() {
	if File_kessel_inventory_v1beta2_streamed_list_objects_request_proto != nil {
		return
	}
	file_kessel_inventory_v1beta2_request_pagination_proto_init()
	file_kessel_inventory_v1beta2_subject_reference_proto_init()
	file_kessel_inventory_v1beta2_consistency_proto_init()
	file_kessel_inventory_v1beta2_representation_type_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*StreamedListObjectsRequest); i {
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
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes[0].OneofWrappers = []any{}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_goTypes,
		DependencyIndexes: file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_depIdxs,
		MessageInfos:      file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_msgTypes,
	}.Build()
	File_kessel_inventory_v1beta2_streamed_list_objects_request_proto = out.File
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_rawDesc = nil
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_goTypes = nil
	file_kessel_inventory_v1beta2_streamed_list_objects_request_proto_depIdxs = nil
}
