package v1beta2

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/stretchr/testify/assert"
)

func TestStreamedListObjectsRequest_Getters(t *testing.T) {
	typ := &RepresentationType{ResourceType: "host"}
	subject := &SubjectReference{}
	pagination := &RequestPagination{Limit: 10}
	consistency := &Consistency{}
	req := &StreamedListObjectsRequest{
		ObjectType:  typ,
		Relation:    "view",
		Subject:     subject,
		Pagination:  pagination,
		Consistency: consistency,
	}
	assert.Equal(t, typ, req.GetObjectType())
	assert.Equal(t, "view", req.GetRelation())
	assert.Equal(t, subject, req.GetSubject())
	assert.Equal(t, pagination, req.GetPagination())
	assert.Equal(t, consistency, req.GetConsistency())

	// nil receiver
	var reqNil *StreamedListObjectsRequest
	assert.Nil(t, reqNil.GetObjectType())
	assert.Equal(t, "", reqNil.GetRelation())
	assert.Nil(t, reqNil.GetSubject())
	assert.Nil(t, reqNil.GetPagination())
	assert.Nil(t, reqNil.GetConsistency())
}

func TestStreamedListObjectsRequest_Reset(t *testing.T) {
	typ := &RepresentationType{ResourceType: "host"}
	subject := &SubjectReference{}
	pagination := &RequestPagination{Limit: 99}
	consistency := &Consistency{}
	req := &StreamedListObjectsRequest{
		ObjectType:  typ,
		Relation:    "view",
		Subject:     subject,
		Pagination:  pagination,
		Consistency: consistency,
	}
	req.Reset()
	assert.Nil(t, req.GetObjectType())
	assert.Equal(t, "", req.GetRelation())
	assert.Nil(t, req.GetSubject())
	assert.Nil(t, req.GetPagination())
	assert.Nil(t, req.GetConsistency())
}

func TestStreamedListObjectsRequest_String(t *testing.T) {
	typ := &RepresentationType{ResourceType: "host"}
	subject := &SubjectReference{}
	req := &StreamedListObjectsRequest{
		ObjectType: typ,
		Relation:   "view",
		Subject:    subject,
	}
	s := req.String()
	assert.NotEmpty(t, s)
	assert.Contains(t, s, "host")
	assert.Contains(t, s, "view")
}

func TestStreamedListObjectsRequest_ProtoMessage(t *testing.T) {
	var req interface{} = &StreamedListObjectsRequest{}
	_, ok := req.(proto.Message)
	assert.True(t, ok)
}

func TestStreamedListObjectsRequest_ProtoReflect(t *testing.T) {
	typ := &RepresentationType{ResourceType: "host"}
	req := &StreamedListObjectsRequest{ObjectType: typ, Relation: "view"}
	m := req.ProtoReflect()
	assert.NotNil(t, m)
	assert.Equal(t, "view", m.Interface().(*StreamedListObjectsRequest).GetRelation())
}

func TestStreamedListObjectsRequest_JSONRoundTrip_Full(t *testing.T) {
	token := "t"
	req := &StreamedListObjectsRequest{
		ObjectType:  &RepresentationType{ResourceType: "host", ReporterType: &token},
		Relation:    "view",
		Subject:     &SubjectReference{},
		Pagination:  &RequestPagination{Limit: 77, ContinuationToken: &token},
		Consistency: &Consistency{},
	}
	b, err := json.Marshal(req)
	assert.NoError(t, err)
	var out StreamedListObjectsRequest
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "host", out.GetObjectType().GetResourceType())
	assert.Equal(t, "view", out.GetRelation())
	assert.NotNil(t, out.GetSubject())
	assert.NotNil(t, out.GetPagination())
	assert.NotNil(t, out.GetConsistency())
}

func TestStreamedListObjectsRequest_JSONRoundTrip_NilFields(t *testing.T) {
	req := &StreamedListObjectsRequest{
		Relation: "write",
	}
	b, err := json.Marshal(req)
	assert.NoError(t, err)
	var out StreamedListObjectsRequest
	assert.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, "write", out.GetRelation())
	assert.Nil(t, out.GetObjectType())
	assert.Nil(t, out.GetSubject())
	assert.Nil(t, out.GetPagination())
	assert.Nil(t, out.GetConsistency())
}
