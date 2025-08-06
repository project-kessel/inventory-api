package errors

import (
	"errors"
	"strings"
	"testing"
)

// AssertIs checks if the error matches the expected sentinel error using errors.Is
func AssertIs(t *testing.T, got error, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("expected error %v, got %v", want, got)
	}
}

// AssertErrorContains checks if an error occurred and contains the expected substring
// Use this only when you need to check for specific context in wrapped errors
func AssertErrorContains(t *testing.T, err error, expectedSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
		return
	}
	if !strings.Contains(err.Error(), expectedSubstring) {
		t.Errorf("Expected error containing '%s', got: %v", expectedSubstring, err)
	}
}
