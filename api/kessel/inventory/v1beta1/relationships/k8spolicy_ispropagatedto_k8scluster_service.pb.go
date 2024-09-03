// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: kessel/inventory/v1beta1/relationships/k8spolicy_ispropagatedto_k8scluster_service.proto

package relationships

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
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

type CreateK8SPolicyIsPropagatedToK8SClusterRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The resource relationship to create in Kessel Asset Inventory
	K8SpolicyIspropagatedtoK8Scluster *K8SPolicyIsPropagatedToK8SCluster `protobuf:"bytes,1,opt,name=k8spolicy_ispropagatedto_k8scluster,json=k8spolicyIspropagatedtoK8scluster,proto3" json:"k8spolicy_ispropagatedto_k8scluster,omitempty"`
}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterRequest) Reset() {
	*x = CreateK8SPolicyIsPropagatedToK8SClusterRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateK8SPolicyIsPropagatedToK8SClusterRequest) ProtoMessage() {}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateK8SPolicyIsPropagatedToK8SClusterRequest.ProtoReflect.Descriptor instead.
func (*CreateK8SPolicyIsPropagatedToK8SClusterRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{0}
}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterRequest) GetK8SpolicyIspropagatedtoK8Scluster() *K8SPolicyIsPropagatedToK8SCluster {
	if x != nil {
		return x.K8SpolicyIspropagatedtoK8Scluster
	}
	return nil
}

type CreateK8SPolicyIsPropagatedToK8SClusterResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterResponse) Reset() {
	*x = CreateK8SPolicyIsPropagatedToK8SClusterResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateK8SPolicyIsPropagatedToK8SClusterResponse) ProtoMessage() {}

func (x *CreateK8SPolicyIsPropagatedToK8SClusterResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateK8SPolicyIsPropagatedToK8SClusterResponse.ProtoReflect.Descriptor instead.
func (*CreateK8SPolicyIsPropagatedToK8SClusterResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{1}
}

type UpdateK8SPolicyIsPropagatedToK8SClusterRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The resource-relationship to be updated will be defined by
	// \"<reporter_data.reporter_type>:<reporter_instance_id>:<reporter_data.subject_local_resource_id>\"
	// AND \"<reporter_data.reporter_type>:<reporter_instance_id>:<reporter_data.object_local_resource_id>\"
	// from the request body.
	K8SpolicyIspropagatedtoK8Scluster *K8SPolicyIsPropagatedToK8SCluster `protobuf:"bytes,1,opt,name=k8spolicy_ispropagatedto_k8scluster,json=k8spolicyIspropagatedtoK8scluster,proto3" json:"k8spolicy_ispropagatedto_k8scluster,omitempty"`
}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterRequest) Reset() {
	*x = UpdateK8SPolicyIsPropagatedToK8SClusterRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateK8SPolicyIsPropagatedToK8SClusterRequest) ProtoMessage() {}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateK8SPolicyIsPropagatedToK8SClusterRequest.ProtoReflect.Descriptor instead.
func (*UpdateK8SPolicyIsPropagatedToK8SClusterRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{2}
}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterRequest) GetK8SpolicyIspropagatedtoK8Scluster() *K8SPolicyIsPropagatedToK8SCluster {
	if x != nil {
		return x.K8SpolicyIspropagatedtoK8Scluster
	}
	return nil
}

type UpdateK8SPolicyIsPropagatedToK8SClusterResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterResponse) Reset() {
	*x = UpdateK8SPolicyIsPropagatedToK8SClusterResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateK8SPolicyIsPropagatedToK8SClusterResponse) ProtoMessage() {}

func (x *UpdateK8SPolicyIsPropagatedToK8SClusterResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateK8SPolicyIsPropagatedToK8SClusterResponse.ProtoReflect.Descriptor instead.
func (*UpdateK8SPolicyIsPropagatedToK8SClusterResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{3}
}

type DeleteK8SPolicyIsPropagatedToK8SClusterRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The resource-relationship to be deleted will be defined by
	// \"<reporter_data.reporter_type>:<reporter_instance_id>:<reporter_data.subject_local_resource_id>\"
	// AND \"<reporter_data.reporter_type>:<reporter_instance_id>:<reporter_data.object_local_resource_id>\"
	// from the request body.
	K8SpolicyIspropagatedtoK8Scluster *K8SPolicyIsPropagatedToK8SCluster `protobuf:"bytes,1,opt,name=k8spolicy_ispropagatedto_k8scluster,json=k8spolicyIspropagatedtoK8scluster,proto3" json:"k8spolicy_ispropagatedto_k8scluster,omitempty"`
}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterRequest) Reset() {
	*x = DeleteK8SPolicyIsPropagatedToK8SClusterRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteK8SPolicyIsPropagatedToK8SClusterRequest) ProtoMessage() {}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteK8SPolicyIsPropagatedToK8SClusterRequest.ProtoReflect.Descriptor instead.
func (*DeleteK8SPolicyIsPropagatedToK8SClusterRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{4}
}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterRequest) GetK8SpolicyIspropagatedtoK8Scluster() *K8SPolicyIsPropagatedToK8SCluster {
	if x != nil {
		return x.K8SpolicyIspropagatedtoK8Scluster
	}
	return nil
}

type DeleteK8SPolicyIsPropagatedToK8SClusterResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterResponse) Reset() {
	*x = DeleteK8SPolicyIsPropagatedToK8SClusterResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteK8SPolicyIsPropagatedToK8SClusterResponse) ProtoMessage() {}

func (x *DeleteK8SPolicyIsPropagatedToK8SClusterResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteK8SPolicyIsPropagatedToK8SClusterResponse.ProtoReflect.Descriptor instead.
func (*DeleteK8SPolicyIsPropagatedToK8SClusterResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP(), []int{5}
}

var File_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto protoreflect.FileDescriptor

var file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDesc = []byte{
	0x0a, 0x58, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x6c, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2f, 0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c, 0x69,
	0x63, 0x79, 0x5f, 0x69, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x74,
	0x6f, 0x5f, 0x6b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x26, 0x6b, 0x65, 0x73, 0x73,
	0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69,
	0x70, 0x73, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61,
	0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x50, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x6c, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2f, 0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c, 0x69,
	0x63, 0x79, 0x5f, 0x69, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x74,
	0x6f, 0x5f, 0x6b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x22, 0xcc, 0x01, 0x0a, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53,
	0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74,
	0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x99, 0x01, 0x0a, 0x23, 0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c,
	0x69, 0x63, 0x79, 0x5f, 0x69, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64,
	0x74, 0x6f, 0x5f, 0x6b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x49, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76,
	0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72,
	0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x4b, 0x38, 0x53,
	0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74,
	0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x21,
	0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61,
	0x67, 0x61, 0x74, 0x65, 0x64, 0x74, 0x6f, 0x4b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x22, 0x31, 0x0a, 0x2f, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f,
	0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64,
	0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x22, 0xcc, 0x01, 0x0a, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4b,
	0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67,
	0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x99, 0x01, 0x0a, 0x23, 0x6b, 0x38, 0x73, 0x70,
	0x6f, 0x6c, 0x69, 0x63, 0x79, 0x5f, 0x69, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74,
	0x65, 0x64, 0x74, 0x6f, 0x5f, 0x6b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x49, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69,
	0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x4b,
	0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67,
	0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x52, 0x21, 0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x70, 0x72, 0x6f,
	0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x74, 0x6f, 0x4b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x22, 0x31, 0x0a, 0x2f, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53,
	0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74,
	0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0xcc, 0x01, 0x0a, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70,
	0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x99, 0x01, 0x0a, 0x23, 0x6b, 0x38,
	0x73, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x5f, 0x69, 0x73, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67,
	0x61, 0x74, 0x65, 0x64, 0x74, 0x6f, 0x5f, 0x6b, 0x38, 0x73, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x49, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c,
	0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73,
	0x2e, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70,
	0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74,
	0x65, 0x72, 0x52, 0x21, 0x6b, 0x38, 0x73, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x70,
	0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x74, 0x6f, 0x4b, 0x38, 0x73, 0x63, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x22, 0x31, 0x0a, 0x2f, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4b,
	0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67,
	0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xea, 0x07, 0x0a, 0x2e, 0x4b, 0x65, 0x73,
	0x73, 0x65, 0x6c, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72,
	0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75,
	0x73, 0x74, 0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0xbc, 0x02, 0x0a, 0x27,
	0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49,
	0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53,
	0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x56, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c,
	0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73,
	0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79,
	0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38,
	0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x57, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4b,
	0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67,
	0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x60, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x5a,
	0x3a, 0x01, 0x2a, 0x22, 0x55, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74,
	0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x2d, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69,
	0x70, 0x73, 0x2f, 0x6b, 0x38, 0x73, 0x2d, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x2e, 0x69, 0x73,
	0x2d, 0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x74, 0x6f, 0x2d, 0x6b,
	0x38, 0x73, 0x2d, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0xbc, 0x02, 0x0a, 0x27, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73,
	0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x56, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e,
	0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49,
	0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53,
	0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x57,
	0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72,
	0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4b, 0x38,
	0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61,
	0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x60, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x5a, 0x3a,
	0x01, 0x2a, 0x1a, 0x55, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2d, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70,
	0x73, 0x2f, 0x6b, 0x38, 0x73, 0x2d, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x2e, 0x69, 0x73, 0x2d,
	0x70, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x74, 0x6f, 0x2d, 0x6b, 0x38,
	0x73, 0x2d, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0xb9, 0x02, 0x0a, 0x27, 0x44, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50,
	0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x56, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69,
	0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x4b, 0x38, 0x53, 0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73,
	0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43,
	0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x57, 0x2e,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4b, 0x38, 0x53,
	0x50, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x49, 0x73, 0x50, 0x72, 0x6f, 0x70, 0x61, 0x67, 0x61, 0x74,
	0x65, 0x64, 0x54, 0x6f, 0x4b, 0x38, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x5d, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x57, 0x2a, 0x55,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2d,
	0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x2f, 0x6b, 0x38,
	0x73, 0x2d, 0x70, 0x6f, 0x6c, 0x69, 0x63, 0x79, 0x2e, 0x69, 0x73, 0x2d, 0x70, 0x72, 0x6f, 0x70,
	0x61, 0x67, 0x61, 0x74, 0x65, 0x64, 0x2e, 0x74, 0x6f, 0x2d, 0x6b, 0x38, 0x73, 0x2d, 0x63, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x42, 0x8e, 0x01, 0x0a, 0x36, 0x6f, 0x72, 0x67, 0x2e, 0x70, 0x72,
	0x6f, 0x6a, 0x65, 0x63, 0x74, 0x5f, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x61, 0x70, 0x69,
	0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x68, 0x69, 0x70, 0x73,
	0x50, 0x01, 0x5a, 0x52, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70,
	0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x2d, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e,
	0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2d, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x70, 0x69, 0x2f,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79,
	0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x6c, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x68, 0x69, 0x70, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescOnce sync.Once
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescData = file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDesc
)

func file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescGZIP() []byte {
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescOnce.Do(func() {
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescData)
	})
	return file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDescData
}

var file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_goTypes = []any{
	(*CreateK8SPolicyIsPropagatedToK8SClusterRequest)(nil),  // 0: kessel.inventory.v1beta1.relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest
	(*CreateK8SPolicyIsPropagatedToK8SClusterResponse)(nil), // 1: kessel.inventory.v1beta1.relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse
	(*UpdateK8SPolicyIsPropagatedToK8SClusterRequest)(nil),  // 2: kessel.inventory.v1beta1.relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest
	(*UpdateK8SPolicyIsPropagatedToK8SClusterResponse)(nil), // 3: kessel.inventory.v1beta1.relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse
	(*DeleteK8SPolicyIsPropagatedToK8SClusterRequest)(nil),  // 4: kessel.inventory.v1beta1.relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest
	(*DeleteK8SPolicyIsPropagatedToK8SClusterResponse)(nil), // 5: kessel.inventory.v1beta1.relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse
	(*K8SPolicyIsPropagatedToK8SCluster)(nil),               // 6: kessel.inventory.v1beta1.relationships.K8SPolicyIsPropagatedToK8SCluster
}
var file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_depIdxs = []int32{
	6, // 0: kessel.inventory.v1beta1.relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest.k8spolicy_ispropagatedto_k8scluster:type_name -> kessel.inventory.v1beta1.relationships.K8SPolicyIsPropagatedToK8SCluster
	6, // 1: kessel.inventory.v1beta1.relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest.k8spolicy_ispropagatedto_k8scluster:type_name -> kessel.inventory.v1beta1.relationships.K8SPolicyIsPropagatedToK8SCluster
	6, // 2: kessel.inventory.v1beta1.relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest.k8spolicy_ispropagatedto_k8scluster:type_name -> kessel.inventory.v1beta1.relationships.K8SPolicyIsPropagatedToK8SCluster
	0, // 3: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.CreateK8SPolicyIsPropagatedToK8SCluster:input_type -> kessel.inventory.v1beta1.relationships.CreateK8SPolicyIsPropagatedToK8SClusterRequest
	2, // 4: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.UpdateK8SPolicyIsPropagatedToK8SCluster:input_type -> kessel.inventory.v1beta1.relationships.UpdateK8SPolicyIsPropagatedToK8SClusterRequest
	4, // 5: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.DeleteK8SPolicyIsPropagatedToK8SCluster:input_type -> kessel.inventory.v1beta1.relationships.DeleteK8SPolicyIsPropagatedToK8SClusterRequest
	1, // 6: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.CreateK8SPolicyIsPropagatedToK8SCluster:output_type -> kessel.inventory.v1beta1.relationships.CreateK8SPolicyIsPropagatedToK8SClusterResponse
	3, // 7: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.UpdateK8SPolicyIsPropagatedToK8SCluster:output_type -> kessel.inventory.v1beta1.relationships.UpdateK8SPolicyIsPropagatedToK8SClusterResponse
	5, // 8: kessel.inventory.v1beta1.relationships.KesselK8SPolicyIsPropagatedToK8SClusterService.DeleteK8SPolicyIsPropagatedToK8SCluster:output_type -> kessel.inventory.v1beta1.relationships.DeleteK8SPolicyIsPropagatedToK8SClusterResponse
	6, // [6:9] is the sub-list for method output_type
	3, // [3:6] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() {
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_init()
}
func file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_init() {
	if File_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto != nil {
		return
	}
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*CreateK8SPolicyIsPropagatedToK8SClusterRequest); i {
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
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*CreateK8SPolicyIsPropagatedToK8SClusterResponse); i {
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
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*UpdateK8SPolicyIsPropagatedToK8SClusterRequest); i {
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
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*UpdateK8SPolicyIsPropagatedToK8SClusterResponse); i {
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
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteK8SPolicyIsPropagatedToK8SClusterRequest); i {
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
		file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteK8SPolicyIsPropagatedToK8SClusterResponse); i {
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
			RawDescriptor: file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_goTypes,
		DependencyIndexes: file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_depIdxs,
		MessageInfos:      file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_msgTypes,
	}.Build()
	File_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto = out.File
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_rawDesc = nil
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_goTypes = nil
	file_kessel_inventory_v1beta1_relationships_k8spolicy_ispropagatedto_k8scluster_service_proto_depIdxs = nil
}
