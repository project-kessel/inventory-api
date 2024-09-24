package k8spolicies

import (
	"github.com/project-kessel/inventory-api/internal/biz"
)

const (
	ResourceType = "k8s-policy"
)

type K8sPolicyUsecase = biz.DefaultUsecase[K8sPolicy, string]

var New = biz.New[K8sPolicy, string]
