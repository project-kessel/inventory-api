package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
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
	TransactionId              string                 `json:"transaction_id"`
	CreatedAt                  time.Time              `json:"created_at"`
}

// ReporterRepresentationSnapshot is a DTO that mirrors the GORM ReporterRepresentation model structure
type ReporterRepresentationSnapshot struct {
	Representation     RepresentationSnapshot `json:"representation"`
	ReporterResourceID uuid.UUID              `json:"reporter_resource_id"`
	Version            uint                   `json:"version"`
	Generation         uint                   `json:"generation"`
	ReporterVersion    *string                `json:"reporter_version"`
	CommonVersion      uint                   `json:"common_version"`
	TransactionId      string                 `json:"transaction_id"`
	Tombstone          bool                   `json:"tombstone"`
	CreatedAt          time.Time              `json:"created_at"`
}
