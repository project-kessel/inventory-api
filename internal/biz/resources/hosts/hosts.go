package hosts

import (
	"github.com/project-kessel/inventory-api/internal/biz"
)

const (
	ResourceType = "rhel-host"
)

type HostUsecase = biz.DefaultUsecase[Host, string]

var New = biz.New[Host, string]
