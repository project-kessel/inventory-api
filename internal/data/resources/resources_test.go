package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func setupGorm(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.Nil(t, err)

	err = data.Migrate(db, log.NewHelper(log.DefaultLogger))
	require.Nil(t, err)

	return db
}

func resource1() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org",
		ResourceData: map[string]any{
			"foo": "bar",
		},
		ResourceType: "my-resource",
		WorkspaceId:  "my-workspace",
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: model.Labels{
			{
				Key:   "label-1",
				Value: "value-1",
			},
			{
				Key:   "label-1",
				Value: "value-2",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

// Checks for resource equality
// This is required as the time.Time are not exactly equal
func assertEqualResource(t *testing.T, r1 *model.Resource, r2 *model.Resource) {
	assert.Equal(t, r1.CreatedAt.Unix(), r2.CreatedAt.Unix())
	assert.Equal(t, r1.UpdatedAt.Unix(), r2.UpdatedAt.Unix())

	r1b := *r1
	r2b := *r2

	r1b.CreatedAt = nil
	r2b.CreatedAt = nil
	r1b.UpdatedAt = nil
	r2b.UpdatedAt = nil

	assert.Equal(t, r1b, r2b)
}

func assertEqualResourceHistory(t *testing.T, r *model.Resource, rh *model.ResourceHistory, operationType model.OperationType) {
	rhExpected := &model.ResourceHistory{
		ID:            rh.ID,
		OrgId:         r.OrgId,
		ResourceData:  r.ResourceData,
		ResourceType:  r.ResourceType,
		WorkspaceId:   r.WorkspaceId,
		Reporter:      r.Reporter,
		ConsoleHref:   r.ConsoleHref,
		ApiHref:       r.ApiHref,
		Labels:        r.Labels,
		ResourceId:    r.ID,
		Timestamp:     rh.Timestamp,
		OperationType: operationType,
	}

	assert.Equal(t, r.CreatedAt.Unix(), rh.Timestamp.Unix())
	assert.Equal(t, rhExpected, rh)
}

func assertEqualLocalHistoryToResource(t *testing.T, r *model.Resource, litr *model.LocalInventoryToResource) {
	litrExpected := &model.LocalInventoryToResource{
		ResourceId:         r.ID,
		CreatedAt:          litr.CreatedAt,
		ReporterResourceId: model.ReporterResourceIdFromResource(r),
	}

	assert.Equal(t, r.CreatedAt.Unix(), litr.CreatedAt.Unix())
	assert.Equal(t, litrExpected, litr)
}

func TestCreateResource(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// Saving a resource not present in the system saves correctly
	r, err := repo.Save(ctx, resource1())
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// The resource is now in the database and is equal to the return value from Save
	resource := model.Resource{}
	assert.Nil(t, db.First(&resource, r.ID).Error)
	assertEqualResource(t, &resource, r)

	// One resource_history entry is created
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 1)
	assertEqualResourceHistory(t, &resource, &resourceHistory[0], model.OperationTypeCreate)

	// One LocalInventoryToResource mapping is also created
	localInventoryToResource := []model.LocalInventoryToResource{}
	assert.Nil(t, db.Find(&localInventoryToResource).Error)
	assert.Len(t, localInventoryToResource, 1)
	assertEqualLocalHistoryToResource(t, &resource, &localInventoryToResource[0])
}

func TestUpdateFailsIfResourceNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Update fails if id is not found
	_, err = repo.Update(ctx, &model.Resource{}, id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpdateResource(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	r, err := repo.Save(ctx, resource1())
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Updates
	r2Copy := *r
	r2Copy.WorkspaceId = "workspace-update-01"
	r2Copy.OrgId = "org-update-01"
	r2, err := repo.Update(ctx, &r2Copy, r.ID)
	assert.NotNil(t, r2)
	assert.Nil(t, err)
	assert.Equal(t, r.ID, r2.ID)

	// The resource is now in the database and is equal to the return value from Update
	resource := model.Resource{}
	assert.Nil(t, db.First(&resource, r2.ID).Error)
	assertEqualResource(t, &resource, r2)

	// Two resource_history entry are created
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 2)
	assertEqualResourceHistory(t, &resource, &resourceHistory[1], model.OperationTypeUpdate)
}

func TestDeleteFailsIfResourceNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Delete fails if id is not found
	_, err = repo.Delete(ctx, id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestDeleteAfterCreate(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	r, err := repo.Save(ctx, resource1())
	assert.NotNil(t, r)
	assert.Nil(t, err)

	r1del, err := repo.Delete(ctx, r.ID)
	assert.Nil(t, err)
	assertEqualResource(t, r, r1del)

	// resource not found
	assert.ErrorIs(t, db.First(&model.Resource{}, r1del.ID).Error, gorm.ErrRecordNotFound)

	// two history, 1 create, 1 delete
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 2)
	assertEqualResourceHistory(t, r, &resourceHistory[1], model.OperationTypeDelete)
}

func TestDeleteAfterUpdate(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// Create
	r, err := repo.Save(ctx, resource1())
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Updates
	_, err = repo.Update(ctx, resource1(), r.ID)
	assert.Nil(t, err)

	// Delete
	_, err = repo.Delete(ctx, r.ID)
	assert.Nil(t, err)

	// 3 history entries, 1 create, 1 update, 1 delete
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 3)
	assertEqualResourceHistory(t, r, &resourceHistory[2], model.OperationTypeDelete)
}
