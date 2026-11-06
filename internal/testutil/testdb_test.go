package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTestDBBackend verifies that selection and backend branches agree after
// normalization, including the empty-value PostgreSQL default.
func TestTestDBBackend(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "default", value: "", want: BackendPostgres},
		{name: "whitespace default", value: " \t ", want: BackendPostgres},
		{name: "postgres", value: "postgres", want: BackendPostgres},
		{name: "sqlite", value: "sqlite", want: BackendSQLite},
		{name: "normalized postgres", value: " Postgres\t", want: BackendPostgres},
		{name: "normalized sqlite", value: " SQLITE ", want: BackendSQLite},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(TestDBBackendEnvVar, testCase.value)
			require.Equal(t, testCase.want, TestDBBackend())
			require.Equal(t, testCase.want == BackendPostgres, UsingPostgres())
		})
	}
}

// TestTestDBBackend_Invalid verifies that a typo fails before database setup or
// backend-specific branches can silently run SQLite behavior or skip coverage.
func TestTestDBBackend_Invalid(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		selectBackend func(*testing.T)
	}{
		{name: "direct selection", selectBackend: func(t *testing.T) { TestDBBackend() }},
		{name: "backend branch", selectBackend: func(t *testing.T) { UsingPostgres() }},
		{name: "database setup", selectBackend: func(t *testing.T) { NewTestDB(t, nil, nil) }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(TestDBBackendEnvVar, "postgre")
			require.PanicsWithValue(t,
				`unknown TEST_DB value "postgre": expected "postgres" or "sqlite"`,
				func() { testCase.selectBackend(t) })
		})
	}
}
