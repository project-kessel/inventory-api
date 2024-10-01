package k8spolicy

import (
	"github.com/project-kessel/inventory-api/internal/biz"
)

const (
	RelationType = "k8s-policy_ispropagatedto_k8s-cluster"
)

type K8SPolicyIsPropagatedToK8SClusterUsecase = biz.DefaultUsecase[K8SPolicyIsPropagatedToK8SCluster, string]

var New = biz.New[K8SPolicyIsPropagatedToK8SCluster, string]
