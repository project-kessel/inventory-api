package storage

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
)

func testLogger() *log.Helper {
	return log.NewHelper(log.NewStdLogger(io.Discard))
}

func TestNew_DisabledPersistence(t *testing.T) {
	cfg := CompletedConfig{
		&completedConfig{
			Options: &Options{DisablePersistence: true},
		},
	}
	db, err := New(cfg, testLogger())
	assert.NoError(t, err)
	assert.Nil(t, db)
}

func TestNew_UnknownDatabase(t *testing.T) {
	cfg := CompletedConfig{
		&completedConfig{
			Options: &Options{Database: "oracle"},
		},
	}
	db, err := New(cfg, testLogger())
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "unrecognized database type")
}
