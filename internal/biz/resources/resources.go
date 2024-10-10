package resources

import (
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type Usecase = biz.DefaultUsecase[model.Resource, string]

var New = biz.New[model.Resource, string]
