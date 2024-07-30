package hosts

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type ReporterData struct {
	// ReporterID should be populated from the Identity of the caller.  e.g. if this is an ACM reporter, *which* ACM
	// instance is it?
	ReporterID string `gorm:"primaryKey"`

	// This is necessary to satisfy gorm so the collection in the Resource model works.
	HostID int64 `json:"-"`

	// This is the type of the Data blob below.  It specifies whether this is an OCM cluster, an ACM cluster,
	// etc.  It seems reasonable to infer the value from the caller's identity data, but it's not clear that's
	// *always* the case.  So, allow it to be passed explicitly and then log a warning or something if the value
	// doesn't match the inferred type.
	ReporterType string `gorm:"primaryKey"`

	Created time.Time
	Updated time.Time

	// LocalResourceId is the identifier assigned to the resource within the reporter's system.
	LocalResourceId string `gorm:"primaryKey"`

	// The version of the reporter.
	ReporterVersion string

	// pointers to where to access the resource
	ConsoleHref string
	ApiHref     string
}

type Tag struct {
	Key   string
	Value string
}

type Host struct {
	// Kessel Asset Inventory generated identifier.
	ID int64 `gorm:"primaryKey"`

	// The type of the Resource.  This should match the segment of the URL path
	// that is used to manage individual resources of the type; e.g.,
	// \"k8s-cluster\"
	ResourceType string

	// Date and time when the inventory item was first reported.
	FirstReported string

	// Identification of the reporter that first reported this item.
	FirstReportedBy string

	// Date and time when the inventory item was last updated.
	LatestReported string

	// Id of the reporter that last reported on this item.
	LatestReportedBy string

	// The workspace in which this resource is a member for access control.  A
	// resource can only be a member of one workspace.
	Workspace string

	// Write only reporter specific data
	ReporterData []ReporterData
	// The entities that registered this item in the Kessel Asset Inventory. The
	// same resource may be registered by multiple reporters
	Tags []Tag
}

// HostRepo is a Greater repo.
type HostRepo interface {
	Save(context.Context, *Host) (*Host, error)
	Update(context.Context, *Host) (*Host, error)
	Delete(context.Context, int64) error
	FindByID(context.Context, int64) (*Host, error)
	ListAll(context.Context) ([]*Host, error)
}

// HostUsecase is a Host usecase.
type HostUsecase struct {
	repo HostRepo
	log  *log.Helper
}

// NewHostUsecase new a Host usecase.
func New(repo HostRepo, logger log.Logger) *HostUsecase {
	return &HostUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateHost creates a Host, and returns the new Host.
func (uc *HostUsecase) CreateHost(ctx context.Context, h *Host) (*Host, error) {
	if ret, err := uc.repo.Save(ctx, h); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("CreateHost: %v", h.ID)
		return ret, nil
	}
}
