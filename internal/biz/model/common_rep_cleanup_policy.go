package model

import (
	"fmt"
	"strings"
)

// CommonRepCleanupPolicy freezes the per-resource retention rule for one invocation.
// The rule keeps the authoritative current common representation and its keepVersions-1
// predecessors for every resource that still has an active (non-tombstoned) reporter.
type CommonRepCleanupPolicy struct {
	keepVersions int
	resourceType ResourceType
	hasType      bool
	valid        bool
}

// NewCommonRepCleanupPolicy constructs a per-resource retention policy. keepVersions
// must be at least 2 because update consumption reads event version v and v-1.
func NewCommonRepCleanupPolicy(keepVersions int, resourceType string) (CommonRepCleanupPolicy, error) {
	if keepVersions < 2 {
		return CommonRepCleanupPolicy{}, fmt.Errorf("common representation retention must keep at least the current version and its predecessor")
	}
	policy := CommonRepCleanupPolicy{keepVersions: keepVersions, valid: true}
	if strings.TrimSpace(resourceType) != "" {
		validatedResourceType, err := NewResourceType(resourceType)
		if err != nil {
			return CommonRepCleanupPolicy{}, fmt.Errorf("invalid resource type filter: %w", err)
		}
		policy.resourceType = validatedResourceType
		policy.hasType = true
	} else if resourceType != "" {
		return CommonRepCleanupPolicy{}, fmt.Errorf("resource type filter cannot contain only whitespace")
	}
	return policy, nil
}

// Validate rejects the zero value, which is not a constructed cleanup policy.
func (policy CommonRepCleanupPolicy) Validate() error {
	if !policy.valid {
		return fmt.Errorf("common representation cleanup policy is not initialized")
	}
	return nil
}

// ResourceType returns the normalized resource type filter and whether a filter was selected.
func (policy CommonRepCleanupPolicy) ResourceType() (ResourceType, bool) {
	return policy.resourceType, policy.hasType
}

// KeepVersions returns the number of existing versions retained per resource.
func (policy CommonRepCleanupPolicy) KeepVersions() int { return policy.keepVersions }
