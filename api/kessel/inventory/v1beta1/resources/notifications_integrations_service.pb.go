// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        (unknown)
// source: kessel/inventory/v1beta1/resources/notifications_integrations_service.proto

package resources

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

type CreateNotificationsIntegrationRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The NotificationsIntegration to create in Kessel Asset Inventory
	Integration *NotificationsIntegration `protobuf:"bytes,1,opt,name=integration,proto3" json:"integration,omitempty"`
}

func (x *CreateNotificationsIntegrationRequest) Reset() {
	*x = CreateNotificationsIntegrationRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateNotificationsIntegrationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateNotificationsIntegrationRequest) ProtoMessage() {}

func (x *CreateNotificationsIntegrationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateNotificationsIntegrationRequest.ProtoReflect.Descriptor instead.
func (*CreateNotificationsIntegrationRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{0}
}

func (x *CreateNotificationsIntegrationRequest) GetIntegration() *NotificationsIntegration {
	if x != nil {
		return x.Integration
	}
	return nil
}

type CreateNotificationsIntegrationResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *CreateNotificationsIntegrationResponse) Reset() {
	*x = CreateNotificationsIntegrationResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateNotificationsIntegrationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateNotificationsIntegrationResponse) ProtoMessage() {}

func (x *CreateNotificationsIntegrationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateNotificationsIntegrationResponse.ProtoReflect.Descriptor instead.
func (*CreateNotificationsIntegrationResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{1}
}

type UpdateNotificationsIntegrationRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The resource to be updated will be defined by
	// \"<reporter_data.reporter_type>:<reporter_instaance_id>:<reporter_data.local_resource_id>\"
	// from the request body.
	Integration *NotificationsIntegration `protobuf:"bytes,1,opt,name=integration,proto3" json:"integration,omitempty"`
}

func (x *UpdateNotificationsIntegrationRequest) Reset() {
	*x = UpdateNotificationsIntegrationRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateNotificationsIntegrationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateNotificationsIntegrationRequest) ProtoMessage() {}

func (x *UpdateNotificationsIntegrationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateNotificationsIntegrationRequest.ProtoReflect.Descriptor instead.
func (*UpdateNotificationsIntegrationRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{2}
}

func (x *UpdateNotificationsIntegrationRequest) GetIntegration() *NotificationsIntegration {
	if x != nil {
		return x.Integration
	}
	return nil
}

type UpdateNotificationsIntegrationResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateNotificationsIntegrationResponse) Reset() {
	*x = UpdateNotificationsIntegrationResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateNotificationsIntegrationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateNotificationsIntegrationResponse) ProtoMessage() {}

func (x *UpdateNotificationsIntegrationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateNotificationsIntegrationResponse.ProtoReflect.Descriptor instead.
func (*UpdateNotificationsIntegrationResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{3}
}

type DeleteNotificationsIntegrationRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The resource to be deleted will be defined by
	// \"<reporter_data.reporter_type>:<reporter_instaance_id>:<reporter_data.local_resource_id>\"
	// from the request body.
	Integration *NotificationsIntegration `protobuf:"bytes,1,opt,name=integration,proto3" json:"integration,omitempty"`
}

func (x *DeleteNotificationsIntegrationRequest) Reset() {
	*x = DeleteNotificationsIntegrationRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteNotificationsIntegrationRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteNotificationsIntegrationRequest) ProtoMessage() {}

func (x *DeleteNotificationsIntegrationRequest) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteNotificationsIntegrationRequest.ProtoReflect.Descriptor instead.
func (*DeleteNotificationsIntegrationRequest) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{4}
}

func (x *DeleteNotificationsIntegrationRequest) GetIntegration() *NotificationsIntegration {
	if x != nil {
		return x.Integration
	}
	return nil
}

type DeleteNotificationsIntegrationResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *DeleteNotificationsIntegrationResponse) Reset() {
	*x = DeleteNotificationsIntegrationResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteNotificationsIntegrationResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteNotificationsIntegrationResponse) ProtoMessage() {}

func (x *DeleteNotificationsIntegrationResponse) ProtoReflect() protoreflect.Message {
	mi := &file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteNotificationsIntegrationResponse.ProtoReflect.Descriptor instead.
func (*DeleteNotificationsIntegrationResponse) Descriptor() ([]byte, []int) {
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP(), []int{5}
}

var File_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto protoreflect.FileDescriptor

var file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDesc = []byte{
	0x0a, 0x4b, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x73, 0x2f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x5f,
	0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x22, 0x6b,
	0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x73, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e,
	0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x42, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72,
	0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x73, 0x2f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x87, 0x01, 0x0a, 0x25, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4e, 0x6f,
	0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x5e, 0x0a,
	0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x3c, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65,
	0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65,
	0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x2e, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x28, 0x0a,
	0x26, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x87, 0x01, 0x0a, 0x25, 0x55, 0x70, 0x64, 0x61,
	0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49,
	0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x5e, 0x0a, 0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x3c, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e,
	0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x2e, 0x4e, 0x6f, 0x74, 0x69,
	0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x22, 0x28, 0x0a, 0x26, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66,
	0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x87, 0x01, 0x0a, 0x25,
	0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x5e, 0x0a, 0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x3c, 0x2e, 0x6b, 0x65, 0x73,
	0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x2e,
	0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74,
	0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0b, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x28, 0x0a, 0x26, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4e,
	0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65,
	0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32,
	0xad, 0x06, 0x0a, 0x25, 0x4b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69,
	0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0xff, 0x01, 0x0a, 0x1e, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x49, 0x2e, 0x6b,
	0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x73, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x4a, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c,
	0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x2e, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x46, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x40, 0x3a, 0x01, 0x2a, 0x22, 0x3b,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73,
	0x2f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2d, 0x69,
	0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0xff, 0x01, 0x0a, 0x1e,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x49,
	0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72,
	0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69,
	0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x4a, 0x2e, 0x6b, 0x65, 0x73, 0x73,
	0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x2e, 0x55,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x46, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x40, 0x3a, 0x01, 0x2a,
	0x1a, 0x3b, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79,
	0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x73, 0x2f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2d, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0xff, 0x01,
	0x0a, 0x1e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x49, 0x2e, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74,
	0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x73, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69,
	0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x4a, 0x2e, 0x6b, 0x65,
	0x73, 0x73, 0x65, 0x6c, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73,
	0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x4e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x49, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x46, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x40, 0x3a,
	0x01, 0x2a, 0x2a, 0x3b, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f,
	0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x73, 0x2f, 0x6e, 0x6f, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x73, 0x2d, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42,
	0x86, 0x01, 0x0a, 0x32, 0x6f, 0x72, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x5f,
	0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2e, 0x61, 0x70, 0x69, 0x2e, 0x69, 0x6e, 0x76, 0x65, 0x6e,
	0x74, 0x6f, 0x72, 0x79, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x72, 0x65, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x50, 0x01, 0x5a, 0x4e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x2d, 0x6b, 0x65, 0x73,
	0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76, 0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2d, 0x61, 0x70,
	0x69, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6b, 0x65, 0x73, 0x73, 0x65, 0x6c, 0x2f, 0x69, 0x6e, 0x76,
	0x65, 0x6e, 0x74, 0x6f, 0x72, 0x79, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x72,
	0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescOnce sync.Once
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescData = file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDesc
)

func file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescGZIP() []byte {
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescOnce.Do(func() {
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescData)
	})
	return file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDescData
}

var file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_goTypes = []any{
	(*CreateNotificationsIntegrationRequest)(nil),  // 0: kessel.inventory.v1beta1.resources.CreateNotificationsIntegrationRequest
	(*CreateNotificationsIntegrationResponse)(nil), // 1: kessel.inventory.v1beta1.resources.CreateNotificationsIntegrationResponse
	(*UpdateNotificationsIntegrationRequest)(nil),  // 2: kessel.inventory.v1beta1.resources.UpdateNotificationsIntegrationRequest
	(*UpdateNotificationsIntegrationResponse)(nil), // 3: kessel.inventory.v1beta1.resources.UpdateNotificationsIntegrationResponse
	(*DeleteNotificationsIntegrationRequest)(nil),  // 4: kessel.inventory.v1beta1.resources.DeleteNotificationsIntegrationRequest
	(*DeleteNotificationsIntegrationResponse)(nil), // 5: kessel.inventory.v1beta1.resources.DeleteNotificationsIntegrationResponse
	(*NotificationsIntegration)(nil),               // 6: kessel.inventory.v1beta1.resources.NotificationsIntegration
}
var file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_depIdxs = []int32{
	6, // 0: kessel.inventory.v1beta1.resources.CreateNotificationsIntegrationRequest.integration:type_name -> kessel.inventory.v1beta1.resources.NotificationsIntegration
	6, // 1: kessel.inventory.v1beta1.resources.UpdateNotificationsIntegrationRequest.integration:type_name -> kessel.inventory.v1beta1.resources.NotificationsIntegration
	6, // 2: kessel.inventory.v1beta1.resources.DeleteNotificationsIntegrationRequest.integration:type_name -> kessel.inventory.v1beta1.resources.NotificationsIntegration
	0, // 3: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.CreateNotificationsIntegration:input_type -> kessel.inventory.v1beta1.resources.CreateNotificationsIntegrationRequest
	2, // 4: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.UpdateNotificationsIntegration:input_type -> kessel.inventory.v1beta1.resources.UpdateNotificationsIntegrationRequest
	4, // 5: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.DeleteNotificationsIntegration:input_type -> kessel.inventory.v1beta1.resources.DeleteNotificationsIntegrationRequest
	1, // 6: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.CreateNotificationsIntegration:output_type -> kessel.inventory.v1beta1.resources.CreateNotificationsIntegrationResponse
	3, // 7: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.UpdateNotificationsIntegration:output_type -> kessel.inventory.v1beta1.resources.UpdateNotificationsIntegrationResponse
	5, // 8: kessel.inventory.v1beta1.resources.KesselNotificationsIntegrationService.DeleteNotificationsIntegration:output_type -> kessel.inventory.v1beta1.resources.DeleteNotificationsIntegrationResponse
	6, // [6:9] is the sub-list for method output_type
	3, // [3:6] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_init() }
func file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_init() {
	if File_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto != nil {
		return
	}
	file_kessel_inventory_v1beta1_resources_notifications_integration_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*CreateNotificationsIntegrationRequest); i {
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
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*CreateNotificationsIntegrationResponse); i {
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
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[2].Exporter = func(v any, i int) any {
			switch v := v.(*UpdateNotificationsIntegrationRequest); i {
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
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[3].Exporter = func(v any, i int) any {
			switch v := v.(*UpdateNotificationsIntegrationResponse); i {
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
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[4].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteNotificationsIntegrationRequest); i {
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
		file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes[5].Exporter = func(v any, i int) any {
			switch v := v.(*DeleteNotificationsIntegrationResponse); i {
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
			RawDescriptor: file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_goTypes,
		DependencyIndexes: file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_depIdxs,
		MessageInfos:      file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_msgTypes,
	}.Build()
	File_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto = out.File
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_rawDesc = nil
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_goTypes = nil
	file_kessel_inventory_v1beta1_resources_notifications_integrations_service_proto_depIdxs = nil
}
