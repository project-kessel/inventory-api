package middleware

import (
	"github.com/go-kratos/kratos/v2/log"
	"os"
)

const defaultResourceDir = "data/resources"

var (
	AllowedResourceTypes = map[string]struct{}{}
)

func LoadResources() {
	if resourceDir == "" {
		resourceDir = defaultResourceDir
	}
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		log.Fatalf("Failed to read resource directory %s: %v", resourceDir, err)
	}
	log.Infof("Read resource directory %s:", resourceDir)

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}

		AllowedResourceTypes[dir.Name()] = struct{}{}
	}
}
