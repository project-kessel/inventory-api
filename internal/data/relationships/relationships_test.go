package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/data/resources"
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

var (
	orgId                  = "my-org"
	workspace              = "workspace-01"
	reporterId             = "my-reporter-id"
	reporterType           = "my-reporter-type"
	subjectLocalResourceId = "software-01"
	objectLocalResourceId  = "heart-hemorrhage"
)

func resourceSubject() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: orgId,
		ResourceData: map[string]any{
			"version": "11.33.12",
		},
		ResourceType: "software",
		WorkspaceId:  workspace,
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: subjectLocalResourceId,
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels:      model.Labels{},
	}
}

func resourceObject() *model.Resource {
	return &model.Resource{
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
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: objectLocalResourceId,
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels:      model.Labels{},
	}
}

func relationship1(subjectId, objectId uuid.UUID) *model.Relationship {
	return &model.Relationship{
		ID:               uuid.UUID{},
		OrgId:            orgId,
		RelationshipData: nil,
		RelationshipType: "software_has-a-bug_bug",
		SubjectId:        subjectId,
		ObjectId:         objectId,
		Reporter: model.RelationshipReporter{
			Reporter: model.Reporter{
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
func assertEqualRelationship(t *testing.T, r1 *model.Relationship, r2 *model.Relationship) {
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

func assertEqualRelationshipHistory(t *testing.T, r *model.Relationship, rh *model.RelationshipHistory, operationType model.OperationType) {
	rhExpected := &model.RelationshipHistory{
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

//func assertEqualLocalHistoryToResource(t *testing.T, r *model.Relationship, litr *model.LocalInventoryToResource) {
//	litrExpected := &model.LocalInventoryToResource{
//		ResourceId:         r.ID,
//		CreatedAt:          litr.CreatedAt,
//		ReporterResourceId: model.ReporterResourceIdFromResource(r),
//	}
//
//	assert.Equal(t, r.CreatedAt.Unix(), litr.CreatedAt.Unix())
//	assert.Equal(t, litrExpected, litr)
//}

func createResource(t *testing.T, db *gorm.DB, resource *model.Resource) uuid.UUID {
	res, err := resources.New(db).Save(context.TODO(), resource)
	assert.Nil(t, err)
	return res.ID
}

func TestCreateRelationship(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, resourceSubject())
	objectId := createResource(t, db, resourceObject())

	// Saving a relationship not present in the system saves correctly
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	// The resource is now in the database and is equal to the return value from Save
	relationship := model.Relationship{}
	assert.Nil(t, db.First(&relationship, r.ID).Error)
	assertEqualRelationship(t, &relationship, r)

	// One relationship_history entry is created
	relationshipHistory := []model.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationshipHistory).Error)
	assert.Len(t, relationshipHistory, 1)
	assertEqualRelationshipHistory(t, &relationship, &relationshipHistory[0], model.OperationTypeCreate)
}

func TestCreateRelationshipFailsIfEitherResourceIsNotFound(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	// Saving a relationship without subject or object fails
	_, err := repo.Save(ctx, relationship1(uuid.Nil, uuid.Nil))
	assert.Error(t, err)

	_, err = repo.Save(ctx, relationship1(uuid.Nil, uuid.Nil))
	assert.Error(t, err)

	// Only subject
	subjectId := createResource(t, db, resourceSubject())
	_, err = repo.Save(ctx, relationship1(subjectId, uuid.Nil))
	assert.Error(t, err)

	// Only object
	db = setupGorm(t)
	repo = New(db)
	objectId := createResource(t, db, resourceSubject())
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
	_, err = repo.Update(ctx, &model.Relationship{}, id)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpdateResource(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, resourceSubject())
	objectId := createResource(t, db, resourceObject())
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
	relationship := model.Relationship{}
	assert.Nil(t, db.First(&relationship, r2.ID).Error)
	assertEqualRelationship(t, &relationship, r2)

	// Two resource_history entry are created
	relationHistory := []model.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationHistory).Error)
	assert.Len(t, relationHistory, 2)
	assertEqualRelationshipHistory(t, &relationship, &relationHistory[1], model.OperationTypeUpdate)
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

	subjectId := createResource(t, db, resourceSubject())
	objectId := createResource(t, db, resourceObject())
	r, err := repo.Save(ctx, relationship1(subjectId, objectId))
	assert.NotNil(t, r)
	assert.Nil(t, err)

	r1del, err := repo.Delete(ctx, r.ID)
	assert.Nil(t, err)
	assertEqualRelationship(t, r, r1del)

	// resource not found
	assert.ErrorIs(t, db.First(&model.Relationship{}, r1del.ID).Error, gorm.ErrRecordNotFound)

	// two history, 1 create, 1 delete
	relationHistory := []model.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationHistory).Error)
	assert.Len(t, relationHistory, 2)
	assertEqualRelationshipHistory(t, r, &relationHistory[1], model.OperationTypeDelete)
}

func TestDeleteAfterUpdate(t *testing.T) {
	db := setupGorm(t)
	repo := New(db)
	ctx := context.TODO()

	subjectId := createResource(t, db, resourceSubject())
	objectId := createResource(t, db, resourceObject())

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
	relationshipHistory := []model.RelationshipHistory{}
	assert.Nil(t, db.Find(&relationshipHistory).Error)
	assert.Len(t, relationshipHistory, 3)
	assertEqualRelationshipHistory(t, r, &relationshipHistory[2], model.OperationTypeDelete)
}
