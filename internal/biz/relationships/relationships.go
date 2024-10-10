package relationships

import (
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type Usecase = biz.DefaultUsecase[model.Relationship, string]

var New = biz.New[model.Relationship, string]
