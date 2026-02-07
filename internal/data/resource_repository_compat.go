package data

// This file provides backward-compatible re-exports for types and functions that
// have been moved to internal/infrastructure/resourcerepository/ and its sub-packages.
// It allows existing importers of internal/data to continue working while the codebase
// is migrated to the new package structure. Remove these once all import sites have
// been updated.

import (
	"github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository"
	"github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/memory"
)

// Type aliases.
type ResourceRepository = resourcerepository.ResourceRepository
type FakeStore = memory.FakeStore

// Function re-exports — shared.
var GetCurrentAndPreviousWorkspaceID = resourcerepository.GetCurrentAndPreviousWorkspaceID

// Function re-exports — in-memory (fake) implementation.
var NewFakeResourceRepository = memory.NewFakeResourceRepository
var NewFakeStore = memory.NewFakeStore
