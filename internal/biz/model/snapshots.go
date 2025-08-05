package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

// RepresentationSnapshot is a DTO for the representation data structure
type RepresentationSnapshot struct {
	Data internal.JsonObject `json:"data"`
}

// ResourceSnapshot is a DTO that mirrors the GORM Resource model structure
type ResourceSnapshot struct {
	ID               uuid.UUID `json:"id"`
	Type             string    `json:"type"`
	CommonVersion    uint      `json:"common_version"`
	ConsistencyToken string    `json:"consistency_token"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ReporterResourceKeySnapshot is a DTO that mirrors the GORM ReporterResourceKey structure
type ReporterResourceKeySnapshot struct {
	LocalResourceID    string `json:"local_resource_id"`
	ReporterType       string `json:"reporter_type"`
	ResourceType       string `json:"resource_type"`
	ReporterInstanceID string `json:"reporter_instance_id"`
}

// ReporterResourceSnapshot is a DTO that mirrors the GORM ReporterResource model structure
type ReporterResourceSnapshot struct {
	ID                    uuid.UUID                   `json:"id"`
	ReporterResourceKey   ReporterResourceKeySnapshot `json:"reporter_resource_key"`
	ResourceID            uuid.UUID                   `json:"resource_id"`
	APIHref               string                      `json:"api_href"`
	ConsoleHref           string                      `json:"console_href"`
	RepresentationVersion uint                        `json:"representation_version"`
	Generation            uint                        `json:"generation"`
	Tombstone             bool                        `json:"tombstone"`
	CreatedAt             time.Time                   `json:"created_at"`
	UpdatedAt             time.Time                   `json:"updated_at"`
}

// CommonRepresentationSnapshot is a DTO that mirrors the GORM CommonRepresentation model structure
type CommonRepresentationSnapshot struct {
	Representation             RepresentationSnapshot `json:"representation"`
	ResourceId                 uuid.UUID              `json:"resource_id"`
	Version                    uint                   `json:"version"`
	ReportedByReporterType     string                 `json:"reported_by_reporter_type"`
	ReportedByReporterInstance string                 `json:"reported_by_reporter_instance"`
	CreatedAt                  time.Time              `json:"created_at"`
}

// ReporterRepresentationSnapshot is a DTO that mirrors the GORM ReporterRepresentation model structure
type ReporterRepresentationSnapshot struct {
	Representation     RepresentationSnapshot `json:"representation"`
	ReporterResourceID string                 `json:"reporter_resource_id"`
	Version            uint                   `json:"version"`
	Generation         uint                   `json:"generation"`
	ReporterVersion    *string                `json:"reporter_version"`
	CommonVersion      uint                   `json:"common_version"`
	Tombstone          bool                   `json:"tombstone"`
	CreatedAt          time.Time              `json:"created_at"`
}

// Conversion functions from GORM models to snapshot DTOs

// NewRepresentationSnapshot creates a snapshot from a GORM Representation
func NewRepresentationSnapshot(rep datamodel.Representation) RepresentationSnapshot {
	return RepresentationSnapshot{
		Data: rep.Data,
	}
}

// NewResourceSnapshot creates a snapshot from a GORM Resource
func NewResourceSnapshot(res *datamodel.Resource) ResourceSnapshot {
	return ResourceSnapshot{
		ID:               res.ID,
		Type:             res.Type,
		CommonVersion:    res.CommonVersion,
		ConsistencyToken: res.ConsistencyToken,
		CreatedAt:        res.CreatedAt,
		UpdatedAt:        res.UpdatedAt,
	}
}

// NewReporterResourceKeySnapshot creates a snapshot from a GORM ReporterResourceKey
func NewReporterResourceKeySnapshot(key datamodel.ReporterResourceKey) ReporterResourceKeySnapshot {
	return ReporterResourceKeySnapshot{
		LocalResourceID:    key.LocalResourceID,
		ReporterType:       key.ReporterType,
		ResourceType:       key.ResourceType,
		ReporterInstanceID: key.ReporterInstanceID,
	}
}

// NewReporterResourceSnapshot creates a snapshot from a GORM ReporterResource
func NewReporterResourceSnapshot(rr *datamodel.ReporterResource) ReporterResourceSnapshot {
	return ReporterResourceSnapshot{
		ID:                    rr.ID,
		ReporterResourceKey:   NewReporterResourceKeySnapshot(rr.ReporterResourceKey),
		ResourceID:            rr.ResourceID,
		APIHref:               rr.APIHref,
		ConsoleHref:           rr.ConsoleHref,
		RepresentationVersion: rr.RepresentationVersion,
		Generation:            rr.Generation,
		Tombstone:             rr.Tombstone,
		CreatedAt:             rr.CreatedAt,
		UpdatedAt:             rr.UpdatedAt,
	}
}

// NewCommonRepresentationSnapshot creates a snapshot from a GORM CommonRepresentation
func NewCommonRepresentationSnapshot(cr *datamodel.CommonRepresentation) CommonRepresentationSnapshot {
	return CommonRepresentationSnapshot{
		Representation:             NewRepresentationSnapshot(cr.Representation),
		ResourceId:                 cr.ResourceId,
		Version:                    cr.Version,
		ReportedByReporterType:     cr.ReportedByReporterType,
		ReportedByReporterInstance: cr.ReportedByReporterInstance,
		CreatedAt:                  cr.CreatedAt,
	}
}

// NewReporterRepresentationSnapshot creates a snapshot from a GORM ReporterRepresentation
func NewReporterRepresentationSnapshot(rr *datamodel.ReporterRepresentation) ReporterRepresentationSnapshot {
	return ReporterRepresentationSnapshot{
		Representation:     NewRepresentationSnapshot(rr.Representation),
		ReporterResourceID: rr.ReporterResourceID,
		Version:            rr.Version,
		Generation:         rr.Generation,
		ReporterVersion:    rr.ReporterVersion,
		CommonVersion:      rr.CommonVersion,
		Tombstone:          rr.Tombstone,
		CreatedAt:          rr.CreatedAt,
	}
}
