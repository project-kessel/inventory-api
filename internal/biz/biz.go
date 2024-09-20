package biz

import (
	"github.com/google/wire"

	"github.com/project-kessel/inventory-api/internal/biz/hosts"
	"github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	"github.com/project-kessel/inventory-api/internal/biz/k8spolicies"
	"github.com/project-kessel/inventory-api/internal/biz/relationships"
)

var (
	// ProviderSet is biz providers.
	ProviderSet = wire.NewSet(
		hosts.New,
		k8sclusters.New,
		k8spolicies.New,
		relationships.New,
	)
)
