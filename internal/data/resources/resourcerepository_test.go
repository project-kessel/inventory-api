package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

const (
	namespace = "foobar"
	emptyTxId = ""
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
		Reporter:      r.Reporter, //nolint:staticcheck
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
	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
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

	// One InventoryResource mapping is created
	inventoryResource := []model.InventoryResource{}
	assert.Nil(t, db.Find(&inventoryResource).Error)
	assert.Len(t, inventoryResource, 1)
	assert.Equal(t, *resource.InventoryId, inventoryResource[0].ID)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestCreateResourceWithInventoryId(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()
	res1 := resource1()
	res2 := resource1()
	res2.ID, _ = uuid.NewV7()

	r, err := repo.Create(ctx, res1, namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	resource1 := model.Resource{}
	assert.Nil(t, db.First(&resource1, r.ID).Error)
	assertEqualResource(t, &resource1, r)

	// Assign the inventory ID from the first resource to the second resource
	res2.InventoryId = r.InventoryId
	// Force workspace update
	res2.WorkspaceId = "workspace-02"
	res2.ReporterInstanceId = "345"

	r2, err := repo.Create(ctx, res2, namespace, emptyTxId)
	assert.NotNil(t, r2)
	assert.Nil(t, err)

	resource2 := model.Resource{}
	assert.Nil(t, db.First(&resource2, r2.ID).Error)
	assertEqualResource(t, &resource2, r2)
	assert.Nil(t, db.First(&resource1, r.ID).Error)
	// Workspace for resource1 was updated to resource2's workspace
	assert.Equal(t, resource2.WorkspaceId, resource1.WorkspaceId)

	// Only one InventoryResource record still exists, and both records point to it
	inventoryResource := []model.InventoryResource{}
	assert.Nil(t, db.Find(&inventoryResource).Error)
	assert.Len(t, inventoryResource, 1)
	assert.Equal(t, *resource1.InventoryId, inventoryResource[0].ID)
	assert.Equal(t, *resource2.InventoryId, inventoryResource[0].ID)
	// Workspace for InventoryResource was updated to resource2's workspace
	assert.Equal(t, resource2.WorkspaceId, inventoryResource[0].WorkspaceId)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestUpdateFailsIfResourceNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Update fails if id is not found
	_, err = repo.Update(ctx, &model.Resource{}, id, namespace, emptyTxId)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestUpdateResource(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)
	createdAt := r.CreatedAt

	// Updates
	r2Copy := *r
	r2Copy.WorkspaceId = "workspace-update-01"
	r2Copy.OrgId = "org-update-01"
	r2, err := repo.Update(ctx, &r2Copy, r.ID, namespace, emptyTxId)
	assert.NotNil(t, r2)
	assert.Nil(t, err)
	assert.Equal(t, r.ID, r2.ID)
	assert.Equal(t, createdAt.Unix(), r2.CreatedAt.Unix())

	// The resource is now in the database and is equal to the return value from Update
	resource := model.Resource{}
	assert.Nil(t, db.First(&resource, r2.ID).Error)
	assertEqualResource(t, &resource, r2)

	// Two resource_history entry are created
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 2)
	assertEqualResourceHistory(t, &resource, &resourceHistory[1], model.OperationTypeUpdate)

	inventoryResource := []model.InventoryResource{}
	assert.Nil(t, db.Find(&inventoryResource).Error)
	assert.Len(t, inventoryResource, 1)
	// Workspace for InventoryResource was updated to r2's workspace
	assert.Equal(t, r2.WorkspaceId, inventoryResource[0].WorkspaceId)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestDeleteFailsIfResourceNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Delete fails if id is not found
	_, err = repo.Delete(ctx, id, namespace)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestDeleteAfterCreate(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Ensure InventoryResource is created
	inventoryResource := []model.InventoryResource{}
	var count int64
	assert.Nil(t, db.Find(&inventoryResource).Count(&count).Error)
	assert.Equal(t, int64(1), count)

	r1del, err := repo.Delete(ctx, r.ID, namespace)
	assert.Nil(t, err)
	assertEqualResource(t, r, r1del)

	// resource not found
	assert.ErrorIs(t, db.First(&model.Resource{}, r1del.ID).Error, gorm.ErrRecordNotFound)

	// two history, 1 create, 1 delete
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 2)
	assertEqualResourceHistory(t, r, &resourceHistory[1], model.OperationTypeDelete)

	// Ensure InventoryResource is cleaned up
	assert.Nil(t, db.Find(&inventoryResource).Count(&count).Error)
	assert.Equal(t, int64(0), count)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestDeleteAfterUpdate(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// Create
	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Updates
	_, err = repo.Update(ctx, resource1(), r.ID, namespace, emptyTxId)
	assert.Nil(t, err)

	// Delete
	_, err = repo.Delete(ctx, r.ID, namespace)
	assert.Nil(t, err)

	// 3 history entries, 1 create, 1 update, 1 delete
	resourceHistory := []model.ResourceHistory{}
	assert.Nil(t, db.Find(&resourceHistory).Error)
	assert.Len(t, resourceHistory, 3)
	assertEqualResourceHistory(t, r, &resourceHistory[2], model.OperationTypeDelete)

	// Nothing exists in the outbox (expected)
	outboxEvents := []model.OutboxEvent{}
	assert.Nil(t, db.Find(&outboxEvents).Error)
	assert.Len(t, outboxEvents, 0)
}

func TestFindByReporterResourceId(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// Saving a resource not present in the system saves correctly
	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// use nil value ReporterResource Id to check negative case
	reporterResourceId := model.ReporterResourceId{}

	resource, err := repo.FindByReporterResourceId(ctx, reporterResourceId)
	assert.NotNil(t, err)
	assert.Nil(t, resource)

	// check that resource is retrievable via ReporterResourceID object
	reporterResourceId = model.ReporterResourceIdFromResource(r)

	resource, err = repo.FindByReporterResourceId(ctx, reporterResourceId)
	assert.Nil(t, err)
	assert.NotNil(t, resource)
}

func TestFindByReporterData(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()
	res := resource1()
	res.ReporterId = "ACM"
	res.ReporterResourceId = "123"

	// Saving a resource not present in the system saves correctly
	r, err := repo.Create(ctx, res, namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	resource, err := repo.FindByReporterData(ctx, r.ReporterId, r.ReporterResourceId)
	assert.Nil(t, err)
	assert.NotNil(t, resource)

	// check negative case
	resource, err = repo.FindByReporterData(ctx, "random", "random")
	assert.NotNil(t, err)
	assert.Nil(t, resource)
}

func TestFindByWorkspaceId(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()
	res := resource1()
	res.ReporterId = "ACM"
	res.ReporterResourceId = "123"
	res.WorkspaceId = "1234"

	// Saving a resource not present in the system saves correctly
	r, err := repo.Create(ctx, res, namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// find resource we just created by workspace id
	resources, err := repo.FindByWorkspaceId(ctx, "1234")
	assert.Nil(t, err)
	assert.NotEqual(t, []*model.Resource{}, resources)

	// find no resources with workspace id: random
	resources, err = repo.FindByWorkspaceId(ctx, "random")
	assert.Nil(t, err)
	assert.Equal(t, []*model.Resource{}, resources)
}

func TestListAll(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// check negative case without any resources, slice with 0 elements returned
	resources, err := repo.ListAll(ctx)
	assert.Nil(t, err)
	assert.Len(t, resources, 0)

	// create a single resource
	r, err := repo.Create(ctx, resource1(), namespace, emptyTxId)
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// check positive case, a single resource is returned
	resources, err = repo.ListAll(ctx)
	assert.Nil(t, err)
	assert.Len(t, resources, 1)
	assertEqualResource(t, resources[0], r)
}
