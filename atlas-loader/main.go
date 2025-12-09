package main

import (
	"fmt"
	"io"
	"os"

	"ariga.io/atlas-provider-gorm/gormschema"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/data/model"
)

func main() {
	stmts, err := gormschema.New("postgres").Load(
		&model_legacy.OutboxEvent{},
		&model.ReporterRepresentation{},
		&model.CommonRepresentation{},
		&model.ReporterResource{},
		&model.Resource{},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load gorm schema: %v\n", err)
		os.Exit(1)
	}
	io.WriteString(os.Stdout, stmts)
}
