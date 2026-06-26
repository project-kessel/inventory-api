package model

import (
	"errors"
	"fmt"
)

// Violation 1: Exported fields on a value object (should be unexported with accessors)
type ResourceTag struct {
	Key       string
	Value     string
	Namespace ResourceType
}

// Violation 2: No constructor validation — accepts empty strings without error
func NewResourceTag(key, value, namespace string) ResourceTag {
	// Violation 3: Direct type conversion bypassing NewResourceType constructor and its validation/normalization
	rt := ResourceType(namespace)

	return ResourceTag{
		Key:       key,
		Value:     value,
		Namespace: rt,
	}
}

// Violation 4: Error wrapping with %v instead of %w — breaks error chain for sentinel error matching
func ValidateResourceTag(tag ResourceTag) error {
	if tag.Key == "" {
		return fmt.Errorf("validation failed: %v", ErrEmpty)
	}
	if len(tag.Key) > 256 {
		return fmt.Errorf("key too long: %v", ErrTooLong)
	}
	return nil
}

// Violation 5: Returns first error immediately instead of aggregating all validation errors
func ValidateResourceTags(tags []ResourceTag) error {
	for _, tag := range tags {
		if err := ValidateResourceTag(tag); err != nil {
			return err
		}
	}
	return nil
}

// Violation 6: Custom error type instead of using sentinel errors
func FindResourceTag(tags []ResourceTag, key string) (ResourceTag, error) {
	for _, tag := range tags {
		if tag.Key == key {
			return tag, nil
		}
	}
	return ResourceTag{}, errors.New("tag not found")
}
