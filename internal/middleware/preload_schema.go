package middleware

import (
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"sync"
)

var schemaCache sync.Map

func PreloadAllSchemas(resourceDir string) error {
	if viper.GetBool("resources.use_cache") {
		if err := LoadSchemaCacheFromJSON("schema_cache.json"); err != nil {
			log.Errorf("Failed to load schema cache from JSON: %v", err)
			return err
		}
		log.Info("Using JSON cache based on resources directory")
	} else {
		if err := PreloadAllSchemasFromFilesystem(resourceDir); err != nil {
			log.Errorf("Failed to preload schemas from filesystem: %v", err)
			return err
		}
		log.Infof("Using local resources directory: %s", resourceDir)
	}
	return nil
}

func PreloadAllSchemasFromFilesystem(resourceDir string) error {
	if resourceDir == "" {
		resourceDir = viper.GetString("resources.schemaPath")
	}
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return fmt.Errorf("no directories inside schema directory")
	}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType := NormalizeResourceType(dir.Name())

		// Load and store common resource schema
		commonResourceSchema, err := LoadCommonResourceDataSchema(resourceType, resourceDir)
		if err == nil {
			schemaCache.Store(fmt.Sprintf("common:%s", resourceType), commonResourceSchema)
		}

		_, err = loadConfigFile(resourceDir, resourceType)
		if err != nil {
			log.Errorf("Failed to load config file for '%s': %v", resourceType, err)
			return err
		}

		reportersDir := filepath.Join(resourceDir, resourceType, "reporters")
		if _, err := os.Stat(reportersDir); os.IsNotExist(err) {
			continue
		}

		reporterDirs, err := os.ReadDir(reportersDir)
		if err != nil {
			log.Errorf("Failed to read reporters directory for '%s': %v", resourceType, err)
			continue
		}

		for _, reporter := range reporterDirs {
			if !reporter.IsDir() {
				continue
			}
			reporterType := reporter.Name()
			reporterSchema, isReporterSchemaExists, err := LoadResourceSchema(resourceType, reporterType, resourceDir)
			if err == nil && isReporterSchemaExists {
				schemaCache.Store(fmt.Sprintf("%s:%s", resourceType, reporterType), reporterSchema)
			} else {
				log.Warnf("No schema found for %s:%s", resourceType, reporterType)
			}
		}
	}

	schemaCachePath := filepath.Join("./schema_cache.json")
	if err := DumpSchemaCacheToJSON(schemaCachePath); err != nil {
		log.Errorf("Failed to dump schema cache to JSON: %v", err)
		return err
	}
	return nil
}

// LoadSchemaCacheFromJSON loads schema cache from a JSON file
func LoadSchemaCacheFromJSON(filePath string) error {
	jsonData, err := os.ReadFile(filePath)
	defaultSchema := "{\n  \"common:k8s_cluster\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"workspace_id\\\": { \\\"type\\\": \\\"string\\\" }\\n  },\\n  \\\"required\\\": [\\n    \\\"workspace_id\\\"\\n  ]\\n}\\n\\n\",\n  \"common:k8s_policy\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"workspace_id\\\": { \\\"type\\\": \\\"string\\\" }\\n  },\\n  \\\"required\\\": [\\n    \\\"workspace_id\\\"\\n  ]\\n}\\n\\n\",\n  \"common:notifications_integration\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"workspace_id\\\": { \\\"type\\\": \\\"string\\\" }\\n  },\\n  \\\"required\\\": [\\n    \\\"workspace_id\\\"\\n  ]\\n}\\n\\n\",\n  \"common:rhel_host\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"workspace_id\\\": { \\\"type\\\": \\\"string\\\" }\\n  },\\n  \\\"required\\\": [\\n    \\\"workspace_id\\\"\\n  ]\\n}\\n\\n\",\n  \"config:k8s_cluster\": \"cmVzb3VyY2VfdHlwZTogazhzX2NsdXN0ZXIKcmVzb3VyY2VfcmVwb3J0ZXJzOgogIC0gQUNNCiAgLSBBQ1MKICAtIE9DTQo=\",\n  \"config:k8s_policy\": \"cmVzb3VyY2VfdHlwZTogazhzX3BvbGljeQpyZXNvdXJjZV9yZXBvcnRlcnM6CiAgLSBBQ00K\",\n  \"config:notifications_integration\": \"cmVzb3VyY2VfdHlwZTogbm90aWZpY2F0aW9ucy9pbnRlZ3JhdGlvbgpyZXNvdXJjZV9yZXBvcnRlcnM6CiAgLSBOT1RJRklDQVRJT05TCg==\",\n  \"config:rhel_host\": \"cmVzb3VyY2VfdHlwZTogcmhlbF9ob3N0CnJlc291cmNlX3JlcG9ydGVyczoKICAtIEhCSQo=\",\n  \"k8s_cluster:acm\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"external_cluster_id\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cluster_status\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLUSTER_STATUS_UNSPECIFIED\\\",\\n        \\\"CLUSTER_STATUS_OTHER\\\",\\n        \\\"READY\\\",\\n        \\\"FAILED\\\",\\n        \\\"OFFLINE\\\"\\n      ]\\n    },\\n    \\\"cluster_reason\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_vendor\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"KUBE_VENDOR_UNSPECIFIED\\\",\\n        \\\"KUBE_VENDOR_OTHER\\\",\\n        \\\"AKS\\\",\\n        \\\"EKS\\\",\\n        \\\"IKS\\\",\\n        \\\"OPENSHIFT\\\",\\n        \\\"GKE\\\"\\n      ]\\n    },\\n    \\\"vendor_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cloud_platform\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLOUD_PLATFORM_UNSPECIFIED\\\",\\n        \\\"CLOUD_PLATFORM_OTHER\\\",\\n        \\\"NONE_UPI\\\",\\n        \\\"BAREMETAL_IPI\\\",\\n        \\\"BAREMETAL_UPI\\\",\\n        \\\"AWS_IPI\\\",\\n        \\\"AWS_UPI\\\",\\n        \\\"AZURE_IPI\\\",\\n        \\\"AZURE_UPI\\\",\\n        \\\"IBMCLOUD_IPI\\\",\\n        \\\"IBMCLOUD_UPI\\\",\\n        \\\"KUBEVIRT_IPI\\\",\\n        \\\"OPENSTACK_IPI\\\",\\n        \\\"OPENSTACK_UPI\\\",\\n        \\\"GCP_IPI\\\",\\n        \\\"GCP_UPI\\\",\\n        \\\"NUTANIX_IPI\\\",\\n        \\\"NUTANIX_UPI\\\",\\n        \\\"VSPHERE_IPI\\\",\\n        \\\"VSPHERE_UPI\\\",\\n        \\\"OVIRT_IPI\\\"\\n      ]\\n    },\\n    \\\"nodes\\\": {\\n      \\\"type\\\": \\\"array\\\",\\n      \\\"items\\\": {\\n        \\\"type\\\": \\\"object\\\",\\n        \\\"properties\\\": {\\n          \\\"name\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"cpu\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"memory\\\": { \\\"type\\\": \\\"string\\\" }\\n        },\\n        \\\"required\\\": [\\n          \\\"name\\\",\\n          \\\"cpu\\\",\\n          \\\"memory\\\"\\n        ]\\n      }\\n\\n    }\\n  },\\n  \\\"required\\\": [\\n    \\\"external_cluster_id\\\",\\n    \\\"cluster_status\\\",\\n    \\\"cluster_reason\\\",\\n    \\\"kube_version\\\",\\n    \\\"kube_vendor\\\",\\n    \\\"vendor_version\\\",\\n    \\\"cloud_platform\\\"\\n  ]\\n}\\n\\n\",\n  \"k8s_cluster:acs\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"external_cluster_id\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cluster_status\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLUSTER_STATUS_UNSPECIFIED\\\",\\n        \\\"CLUSTER_STATUS_OTHER\\\",\\n        \\\"READY\\\",\\n        \\\"FAILED\\\",\\n        \\\"OFFLINE\\\"\\n      ]\\n    },\\n    \\\"cluster_reason\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_vendor\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"KUBE_VENDOR_UNSPECIFIED\\\",\\n        \\\"KUBE_VENDOR_OTHER\\\",\\n        \\\"AKS\\\",\\n        \\\"EKS\\\",\\n        \\\"IKS\\\",\\n        \\\"OPENSHIFT\\\",\\n        \\\"GKE\\\"\\n      ]\\n    },\\n    \\\"vendor_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cloud_platform\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLOUD_PLATFORM_UNSPECIFIED\\\",\\n        \\\"CLOUD_PLATFORM_OTHER\\\",\\n        \\\"NONE_UPI\\\",\\n        \\\"BAREMETAL_IPI\\\",\\n        \\\"BAREMETAL_UPI\\\",\\n        \\\"AWS_IPI\\\",\\n        \\\"AWS_UPI\\\",\\n        \\\"AZURE_IPI\\\",\\n        \\\"AZURE_UPI\\\",\\n        \\\"IBMCLOUD_IPI\\\",\\n        \\\"IBMCLOUD_UPI\\\",\\n        \\\"KUBEVIRT_IPI\\\",\\n        \\\"OPENSTACK_IPI\\\",\\n        \\\"OPENSTACK_UPI\\\",\\n        \\\"GCP_IPI\\\",\\n        \\\"GCP_UPI\\\",\\n        \\\"NUTANIX_IPI\\\",\\n        \\\"NUTANIX_UPI\\\",\\n        \\\"VSPHERE_IPI\\\",\\n        \\\"VSPHERE_UPI\\\",\\n        \\\"OVIRT_IPI\\\"\\n      ]\\n    },\\n    \\\"nodes\\\": {\\n      \\\"type\\\": \\\"array\\\",\\n      \\\"items\\\": {\\n        \\\"type\\\": \\\"object\\\",\\n        \\\"properties\\\": {\\n          \\\"name\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"cpu\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"memory\\\": { \\\"type\\\": \\\"string\\\" }\\n        },\\n        \\\"required\\\": [\\n          \\\"name\\\",\\n          \\\"cpu\\\",\\n          \\\"memory\\\"\\n        ]\\n      }\\n\\n    }\\n  },\\n  \\\"required\\\": [\\n    \\\"external_cluster_id\\\",\\n    \\\"cluster_status\\\",\\n    \\\"cluster_reason\\\",\\n    \\\"kube_version\\\",\\n    \\\"kube_vendor\\\",\\n    \\\"vendor_version\\\",\\n    \\\"cloud_platform\\\"\\n  ]\\n}\\n\\n\",\n  \"k8s_cluster:ocm\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"external_cluster_id\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cluster_status\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLUSTER_STATUS_UNSPECIFIED\\\",\\n        \\\"CLUSTER_STATUS_OTHER\\\",\\n        \\\"READY\\\",\\n        \\\"FAILED\\\",\\n        \\\"OFFLINE\\\"\\n      ]\\n    },\\n    \\\"cluster_reason\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"kube_vendor\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"KUBE_VENDOR_UNSPECIFIED\\\",\\n        \\\"KUBE_VENDOR_OTHER\\\",\\n        \\\"AKS\\\",\\n        \\\"EKS\\\",\\n        \\\"IKS\\\",\\n        \\\"OPENSHIFT\\\",\\n        \\\"GKE\\\"\\n      ]\\n    },\\n    \\\"vendor_version\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"cloud_platform\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"CLOUD_PLATFORM_UNSPECIFIED\\\",\\n        \\\"CLOUD_PLATFORM_OTHER\\\",\\n        \\\"NONE_UPI\\\",\\n        \\\"BAREMETAL_IPI\\\",\\n        \\\"BAREMETAL_UPI\\\",\\n        \\\"AWS_IPI\\\",\\n        \\\"AWS_UPI\\\",\\n        \\\"AZURE_IPI\\\",\\n        \\\"AZURE_UPI\\\",\\n        \\\"IBMCLOUD_IPI\\\",\\n        \\\"IBMCLOUD_UPI\\\",\\n        \\\"KUBEVIRT_IPI\\\",\\n        \\\"OPENSTACK_IPI\\\",\\n        \\\"OPENSTACK_UPI\\\",\\n        \\\"GCP_IPI\\\",\\n        \\\"GCP_UPI\\\",\\n        \\\"NUTANIX_IPI\\\",\\n        \\\"NUTANIX_UPI\\\",\\n        \\\"VSPHERE_IPI\\\",\\n        \\\"VSPHERE_UPI\\\",\\n        \\\"OVIRT_IPI\\\"\\n      ]\\n    },\\n    \\\"nodes\\\": {\\n      \\\"type\\\": \\\"array\\\",\\n      \\\"items\\\": {\\n        \\\"type\\\": \\\"object\\\",\\n        \\\"properties\\\": {\\n          \\\"name\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"cpu\\\": { \\\"type\\\": \\\"string\\\" },\\n          \\\"memory\\\": { \\\"type\\\": \\\"string\\\" }\\n        },\\n        \\\"required\\\": [\\n          \\\"name\\\",\\n          \\\"cpu\\\",\\n          \\\"memory\\\"\\n        ]\\n      }\\n\\n    }\\n  },\\n  \\\"required\\\": [\\n    \\\"external_cluster_id\\\",\\n    \\\"cluster_status\\\",\\n    \\\"cluster_reason\\\",\\n    \\\"kube_version\\\",\\n    \\\"kube_vendor\\\",\\n    \\\"vendor_version\\\",\\n    \\\"cloud_platform\\\"\\n  ]\\n}\\n\\n\",\n  \"k8s_policy:acm\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"disabled\\\": {\\n      \\\"type\\\": \\\"boolean\\\",\\n      \\\"description\\\": \\\"Defines if the policy is currently enabled or disabled across all targets.\\\"\\n    },\\n    \\\"severity\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"SEVERITY_UNSPECIFIED\\\",\\n        \\\"SEVERITY_OTHER\\\",\\n        \\\"LOW\\\",\\n        \\\"MEDIUM\\\",\\n        \\\"HIGH\\\",\\n        \\\"CRITICAL\\\"\\n      ],\\n      \\\"description\\\": \\\"The severity level of the policy.\\\"\\n    }\\n  },\\n  \\\"required\\\": [\\n    \\\"disabled\\\",\\n    \\\"severity\\\"\\n  ]\\n}\\n\",\n  \"notifications_integration:notifications\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"reporter_type\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"enum\\\": [\\n        \\\"NOTIFICATIONS\\\"\\n      ],\\n      \\\"description\\\": \\\"The type of reporter, fixed to 'NOTIFICATIONS' for this schema.\\\"\\n    },\\n    \\\"reporter_instance_id\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"description\\\": \\\"A unique identifier for the reporter instance, such as a service account.\\\"\\n    },\\n    \\\"local_resource_id\\\": {\\n      \\\"type\\\": \\\"string\\\",\\n      \\\"description\\\": \\\"A string representing the local identifier of the resource.\\\"\\n    }\\n  },\\n  \\\"required\\\": [\\n    \\\"reporter_type\\\",\\n    \\\"reporter_instance_id\\\",\\n    \\\"local_resource_id\\\"\\n  ]\\n}\\n\",\n  \"rhel_host:hbi\": \"{\\n  \\\"$schema\\\": \\\"http://json-schema.org/draft-07/schema#\\\",\\n  \\\"type\\\": \\\"object\\\",\\n  \\\"properties\\\": {\\n    \\\"satellite_id\\\": { \\\"type\\\": \\\"string\\\" },\\n    \\\"sub_manager_id\\\": {\\\"type\\\":  \\\"string\\\"},\\n    \\\"insights_inventory_id\\\": {\\\"type\\\":  \\\"string\\\"},\\n    \\\"ansible_host\\\" : {\\\"type\\\":  \\\"string\\\"}\\n  },\\n  \\\"required\\\": []\\n}\\n\\n\"\n}"
	jsonData, err = os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// If the file does not exist, use the default schema
			log.Info("File not found, using default schema")
			jsonData = []byte(defaultSchema)
		} else {
			return fmt.Errorf("failed to read schema cache file: %w", err)
		}
	}

	cacheMap := make(map[string]interface{})
	err = json.Unmarshal(jsonData, &cacheMap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal schema cache JSON: %w", err)
	}

	for key, value := range cacheMap {
		schemaCache.Store(key, value)
	}

	log.Infof("Schema cache successfully loaded from %s", filePath)
	return nil
}

// Retrieves schema from cache
func getSchemaFromCache(cacheKey string) (string, error) {
	if cachedSchema, ok := schemaCache.Load(cacheKey); ok {
		return cachedSchema.(string), nil
	}
	return "", fmt.Errorf("schema not found for key '%s'", cacheKey)
}

func loadConfigFile(resourceDir string, resourceType string) (struct {
	ResourceType      string   `yaml:"resource_type"`
	ResourceReporters []string `yaml:"resource_reporters"`
}, error) {
	var config struct {
		ResourceType      string   `yaml:"resource_type"`
		ResourceReporters []string `yaml:"resource_reporters"`
	}
	configPath := filepath.Join(resourceDir, resourceType, "config.yaml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file for '%s': %w", resourceType, err)
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return config, fmt.Errorf("failed to unmarshal config for '%s': %w", resourceType, err)
	}
	if config.ResourceReporters == nil {
		return config, fmt.Errorf("missing 'resource_reporters' field in config for '%s'", resourceType)
	}
	configResourceType := NormalizeResourceType(config.ResourceType)
	schemaCache.Store(fmt.Sprintf("config:%s", configResourceType), configData)
	return config, nil
}

// DumpSchemaCacheToJSON saves the schema cache to a JSON file
func DumpSchemaCacheToJSON(filePath string) error {
	cacheMap := make(map[string]interface{})

	schemaCache.Range(func(key, value interface{}) bool {
		cacheMap[key.(string)] = value
		return true
	})

	jsonData, err := json.MarshalIndent(cacheMap, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema cache: %w", err)
	}

	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write schema cache to file: %w", err)
	}

	return nil
}
