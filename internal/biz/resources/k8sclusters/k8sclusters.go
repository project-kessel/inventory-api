package k8sclusters

import (
	"github.com/project-kessel/inventory-api/internal/biz"
)

const (
	ResourceType = "k8s-cluster"
)

type K8sClusterUsecase = biz.DefaultUsecase[K8SCluster, string]

var New = biz.New[K8SCluster, string]
