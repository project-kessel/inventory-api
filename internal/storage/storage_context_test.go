package storage

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNewWithContext_PreCanceledContextDoesNotConnect(t *testing.T) {
	listener := newPostgresListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	db, err := NewWithContext(ctx, postgresCompletedConfig(listener), testLogger())
	require.Nil(t, db)
	require.ErrorIs(t, err, context.Canceled)

	require.NoError(t, listener.SetDeadline(time.Now().Add(500*time.Millisecond)))
	conn, acceptErr := listener.Accept()
	if conn != nil {
		_ = conn.Close()
		t.Fatal("pre-canceled context must not open a database connection")
	}
	var timeoutErr net.Error
	require.ErrorAs(t, acceptErr, &timeoutErr)
	require.True(t, timeoutErr.Timeout())
}

func TestNewWithContext_DeadlineBoundsPostgresStartup(t *testing.T) {
	listener := newPostgresListener(t)
	accepted := watchPostgresStartup(t, listener)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	type openResult struct {
		db  *gorm.DB
		err error
	}
	result := make(chan openResult, 1)
	started := time.Now()
	go func() {
		db, err := NewWithContext(ctx, postgresCompletedConfig(listener), testLogger())
		result <- openResult{db: db, err: err}
	}()

	var opened openResult
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case opened = <-result:
	case <-timer.C:
		t.Fatal("database startup did not return within the context deadline bound")
	}
	if opened.db != nil {
		sqlDB, dbErr := opened.db.DB()
		if dbErr == nil {
			defer func() { _ = sqlDB.Close() }()
		}
	}
	require.Nil(t, opened.db)
	require.ErrorIs(t, opened.err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), 5*time.Second)

	var serverConn net.Conn
	select {
	case serverConn = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("the local PostgreSQL listener did not observe a connection attempt")
	}
	defer func() { _ = serverConn.Close() }()
	require.NoError(t, serverConn.SetReadDeadline(time.Now().Add(2*time.Second)))
	startupBytes, err := io.Copy(io.Discard, serverConn)
	require.NoError(t, err, "failed startup should close its SQL connection")
	require.Greater(t, startupBytes, int64(0), "listener should receive the PostgreSQL startup packet")
}

func TestNewWithContext_UnknownDatabaseReturnsRegularError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cfg := CompletedConfig{
		&completedConfig{
			Options: &Options{Database: "oracle"},
		},
	}

	db, err := NewWithContext(ctx, cfg, testLogger())
	require.Nil(t, db)
	require.ErrorContains(t, err, "unrecognized database type")
	require.NotErrorIs(t, err, context.DeadlineExceeded)
}

func TestNewWithContext_SQLiteQueriesAndUsesCallerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db, err := NewWithContext(ctx, sqliteCompletedConfig(t.Name()), testLogger())
	require.NoError(t, err)
	defer closeStorageDB(t, db)

	assertSQLiteStorageBehavior(t, db)

	cancel()
	var value int
	err = db.Raw("SELECT 1").Scan(&value).Error
	require.ErrorIs(t, err, context.Canceled)
}

func TestNew_SQLiteCompatibility(t *testing.T) {
	db, err := New(sqliteCompletedConfig(t.Name()), testLogger())
	require.NoError(t, err)
	defer closeStorageDB(t, db)

	assertSQLiteStorageBehavior(t, db)
}

func assertSQLiteStorageBehavior(t *testing.T, db *gorm.DB) {
	t.Helper()

	var value int
	require.NoError(t, db.Raw("SELECT 73").Scan(&value).Error)
	require.Equal(t, 73, value)

	type uniqueRecord struct {
		ID   int    `gorm:"primaryKey"`
		Name string `gorm:"uniqueIndex"`
	}
	require.NoError(t, db.AutoMigrate(&uniqueRecord{}))
	require.NoError(t, db.Create(&uniqueRecord{Name: "first"}).Error)
	err := db.Create(&uniqueRecord{Name: "first"}).Error
	require.ErrorIs(t, err, gorm.ErrDuplicatedKey)
}

func closeStorageDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func sqliteCompletedConfig(name string) CompletedConfig {
	return CompletedConfig{
		&completedConfig{
			Options: &Options{Database: "sqlite3"},
			DSN:     fmt.Sprintf("file:%s?mode=memory&cache=shared", name),
		},
	}
}

func newPostgresListener(t *testing.T) *net.TCPListener {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func postgresCompletedConfig(listener *net.TCPListener) CompletedConfig {
	port := listener.Addr().(*net.TCPAddr).Port
	return CompletedConfig{
		&completedConfig{
			Options: &Options{Database: "postgres"},
			DSN:     fmt.Sprintf("host=127.0.0.1 port=%d user=testuser dbname=test sslmode=disable", port),
		},
	}
}

func watchPostgresStartup(t *testing.T, listener *net.TCPListener) <-chan net.Conn {
	t.Helper()
	accepted := make(chan net.Conn, 1)
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		conn, err := listener.AcceptTCP()
		if err == nil {
			accepted <- conn
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case conn := <-accepted:
			_ = conn.Close()
		default:
		}
		<-acceptDone
	})
	return accepted
}
