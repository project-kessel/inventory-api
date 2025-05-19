package resources_test

import (
	"testing"

	"github.com/project-kessel/inventory-api/internal/mocks"
)

func TestMockTypesCompile(t *testing.T) {
	_ = &mocks.MockedReporterResourceRepository{}
	_ = &mocks.MockedInventoryResourceRepository{}
	_ = &mocks.MockedListenManager{}
	_ = &mocks.MockedSubscription{}
}
