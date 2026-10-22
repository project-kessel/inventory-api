package data

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

func TestFindCurrentAndPreviousVersionedRepresentationsUsesCompositePrimaryKey(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)

	repository := NewResourceRepository(db, nil, nil)
	key, err := bizmodel.NewReporterResourceKey(
		"resource-1",
		"host",
		"hbi",
		"instance-1",
	)
	require.NoError(t, err)

	expectedQuery := regexp.QuoteMeta(
		"SELECT cr.data, cr.version, cr.resource_id, cr.reported_by_reporter_type, cr.reported_by_reporter_instance, cr.transaction_id " +
			"FROM reporter_resources rr " +
			"JOIN common_representations cr ON rr.resource_id = cr.resource_id " +
			"WHERE rr.local_resource_id = $1 " +
			"AND rr.resource_type = $2 " +
			"AND rr.reporter_type = $3 " +
			"AND rr.reporter_instance_id = $4 " +
			"AND cr.version IN ($5,$6)",
	)
	mock.ExpectQuery(expectedQuery).
		WithArgs(
			"resource-1",
			"host",
			"hbi",
			"instance-1",
			uint(42),
			uint(41),
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"data",
			"version",
			"resource_id",
			"reported_by_reporter_type",
			"reported_by_reporter_instance",
			"transaction_id",
		}))

	version := bizmodel.NewVersion(42)
	_, _, err = repository.FindCurrentAndPreviousVersionedRepresentations(
		db,
		key,
		&version,
		bizmodel.OperationTypeUpdated,
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
