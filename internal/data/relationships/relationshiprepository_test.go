//go:build enable_resource_repository_tests

package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/data/resources"
)

func setupGorm(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.Nil(t, err)

	err = data.Migrate(db, log.NewHelper(log.DefaultLogger))
	require.Nil(t, err)

	return db
}

func setupMetricsCollector(t *testing.T) *metricscollector.MetricsCollector {
	mc := &metricscollector.MetricsCollector{}
	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
	err := mc.New(meter)
	require.Nil(t, err)
	return mc
}

func setupTest(t *testing.T) (*gorm.DB, *metricscollector.MetricsCollector) {
	db := setupGorm(t)
	mc := setupMetricsCollector(t)
	return db, mc
}

var (
	orgId                  = "my-org"
	workspace              = "workspace-01"
	reporterId             = "my-reporter-id"
	reporterType           = "my-reporter-type"
	subjectLocalResourceId = "software-01"
	objectLocalResourceId  = "heart-hemorrhage"
	emptyTxId              = ""
)

func resourceSubject() *model_legacy.Resource {
	return &model_legacy.Resource{
		ID:    uuid.UUID{},
		OrgId: orgId,
		ResourceData: map[string]any{
			"version": "11.33.12",
		},
		ResourceType: "software",
		WorkspaceId:  workspace,
		Reporter: model_legacy.ResourceReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: subjectLocalResourceId,
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels:      model_legacy.Labels{},
	}
}

func resourceObject() *model_legacy.Resource {
	return &model_legacy.Resource{
		ID:    uuid.UUID{},
		OrgId: orgId,
		ResourceData: map[string]any{
			"ssl_ready": true,
			"valves": []map[string]any{
				{
					"name":   "www.example.com",
					"cpu":    "7500m",
					"memory": "30973224Ki",
					"labels": []map[string]any{
						{
							"key":   "has_monster_gpu",
							"value": "yes",
						},
					},
				},
			},
		},
		ResourceType: "bug",
		WorkspaceId:  workspace,
		Reporter: model_legacy.ResourceReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: objectLocalResourceId,
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels:      model_legacy.Labels{},
	}
}

func relationship1(subjectId, objectId uuid.UUID) *model_legacy.Relationship {
	return &model_legacy.Relationship{
		ID:               uuid.UUID{},
		OrgId:            orgId,
		RelationshipData: nil,
		RelationshipType: "software_has-a-bug_bug",
		SubjectId:        subjectId,
		ObjectId:         objectId,
		Reporter: model_legacy.RelationshipReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "3.14.159",
			},
			SubjectLocalResourceId: subjectLocalResourceId,
			SubjectResourceType:    "software",
			ObjectLocalResourceId:  objectLocalResourceId,
			ObjectResourceType:     "bug",
		},
		CreatedAt: nil,
		UpdatedAt: nil,
	}
}

// Checks for resource equality
// This is required as the time.Time are not exactly equal
func assertEqualRelationship(t *testing.T, r1 *model_legacy.Relationship, r2 *model_legacy.Relationship) {
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

func assertEqualRelationshipHistory(t *testing.T, r *model_legacy.Relationship, rh *model_legacy.RelationshipHistory, operationType model_legacy.OperationType) {
	rhExpected := &model_legacy.RelationshipHistory{
		ID:               rh.ID,
		OrgId:            r.OrgId,
		RelationshipData: r.RelationshipData,
		RelationshipType: r.RelationshipType,
		SubjectId:        r.SubjectId,
		ObjectId:         r.ObjectId,
		Reporter:         r.Reporter,
		Timestamp:        rh.Timestamp,
		RelationshipId:   r.ID,
		OperationType:    operationType,
	}

	assert.Equal(t, r.CreatedAt.Unix(), rh.Timestamp.Unix())
	assert.Equal(t, rhExpected, rh)
}

//func assertEqualLocalHistoryToResource(t *testing.T, r *model_legacy.Relationship, litr *model_legacy.LocalInventoryToResource) {
//	litrExpected := &model_legacy.LocalInventoryToResource{
//		ResourceId:         r.ID,
//		CreatedAt:          litr.CreatedAt,
//		ReporterResourceId: model_legacy.ReporterResourceIdFromResource(r),
//	}
//
//	assert.Equal(t, r.CreatedAt.Unix(), litr.CreatedAt.Unix())
//	assert.Equal(t, litrExpected, litr)
//}

func createResource(t *testing.T, db *gorm.DB, mc *metricscollector.MetricsCollector, resource *model_legacy.Resource) uuid.UUID {
	maxSerializationRetries := 3
	tm := data.NewGormTransactionManager(maxSerializationRetries)
	res, err := resources.New(db, mc, tm).Create(context.TODO(), resource, "foobar-namespace", emptyTxId)
	assert.Nil(t, err)
	return res.ID
}

func TestCreateRelationship(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())

	// Saving a relationship not present in the system saves correctly
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// The resource is now in the database and is equal to the return value from Save
	relationship := model_legacy.Relationship{}
	assert.Nil(t, db.First(&relationship, r.ID).Error)
	assertEqualRelationship(t, &relationship, r)

	// One relationship_history entry is created
	relationshipHistory := []model_legacy.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationshipHistory).Error)
	assert.Len(t, relationshipHistory, 1)
	assertEqualRelationshipHistory(t, &relationship, &relationshipHistory[0], model_legacy.OperationTypeCreate)
}

func TestCreateRelationshipFailsIfEitherResourceIsNotFound(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	// Saving a relationship without subject or object fails
	_, err := repo.Save(ctx, relationship1(uuid.Nil, uuid.Nil))
	assert.Error(t, err)

	_, err = repo.Save(ctx, relationship1(uuid.Nil, uuid.Nil))
	assert.Error(t, err)

	// Only subject
	subjectId := createResource(t, db, mc, resourceSubject())
	_, err = repo.Save(ctx, relationship1(subjectId, uuid.Nil))
	assert.Error(t, err)

	// Only object
	db = setupGorm(t)
	repo = New(db)
	objectId := createResource(t, db, mc, resourceSubject())
	_, err = repo.Save(ctx, relationship1(uuid.Nil, objectId))
	assert.Error(t, err)
}

func TestUpdateFailsIfRelationshipNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Update fails if id is not found
	_, err = repo.Update(ctx, &model_legacy.Relationship{}, id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpdateResource(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Updates
	r2Copy := *r
	r2Copy.OrgId = "org-update-01"
	r2, err := repo.Update(ctx, &r2Copy, r.ID)
	assert.NotNil(t, r2)
	assert.Nil(t, err)
	assert.Equal(t, r.ID, r2.ID)

	// The resource is now in the database and is equal to the return value from Update
	relationship := model_legacy.Relationship{}
	assert.Nil(t, db.First(&relationship, r2.ID).Error)
	assertEqualRelationship(t, &relationship, r2)

	// Two resource_history entry are created
	relationHistory := []model_legacy.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationHistory).Error)
	assert.Len(t, relationHistory, 2)
	assertEqualRelationshipHistory(t, &relationship, &relationHistory[1], model_legacy.OperationTypeUpdate)
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
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	r1del, err := repo.Delete(ctx, r.ID)
	assert.Nil(t, err)
	assertEqualRelationship(t, r, r1del)

	// resource not found
	assert.ErrorIs(t, db.First(&model_legacy.Relationship{}, r1del.ID).Error, gorm.ErrRecordNotFound)

	// two history, 1 create, 1 delete
	relationHistory := []model_legacy.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationHistory).Error)
	assert.Len(t, relationHistory, 2)
	assertEqualRelationshipHistory(t, r, &relationHistory[1], model_legacy.OperationTypeDelete)
}

func TestDeleteAfterUpdate(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())

	// Create
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// Updates
	_, err = repo.Update(ctx, relationship1(subjectId, objectId), r.ID)
	assert.Nil(t, err)

	// Delete
	_, err = repo.Delete(ctx, r.ID)
	assert.Nil(t, err)

	// 3 history entries, 1 create, 1 update, 1 delete
	relationshipHistory := []model_legacy.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationshipHistory).Error)
	assert.Len(t, relationshipHistory, 3)
	assertEqualRelationshipHistory(t, r, &relationshipHistory[2], model_legacy.OperationTypeDelete)
}

func TestFindRelationship(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())

	// Saving a relationship not present in the system saves correctly
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// relationship should be found via IDs and type
	relationship, err := repo.FindRelationship(ctx, subjectId, objectId, "software_has-a-bug_bug")

	assert.Nil(t, err)
	assert.NotNil(t, relationship)
	assertEqualRelationship(t, relationship, r)

	// no relationship should be found if type does not match
	relationship, err = repo.FindRelationship(ctx, subjectId, objectId, "invalid")

	assert.NotNil(t, err)
	assert.Nil(t, relationship)
}

func TestListAll(t *testing.T) {
	db, mc := setupTest(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, mc, resourceSubject())
	objectId := createResource(t, db, mc, resourceObject())

	// first check negative case with zero relationships
	relationships, err := repo.ListAll(ctx)
	assert.Nil(t, err)
	assert.Len(t, relationships, 0)

	// Saving a relationship not present in the system saves correctly
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// ListAll should now return a single relationship
	relationships, err = repo.ListAll(ctx)
	assert.Nil(t, err)
	assert.Len(t, relationships, 1)
	assertEqualRelationship(t, relationships[0], r)
}
