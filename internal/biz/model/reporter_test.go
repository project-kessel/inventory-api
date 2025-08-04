//go:build test

package model

import (
	"strings"
	"testing"
)

func TestReporter_Initialization(t *testing.T) {
	t.Parallel()
	fixture := NewReporterTestFixture()

	t.Run("should create reporter with valid inputs", func(t *testing.T) {
		t.Parallel()

		reporterType, err := NewReporterType(fixture.ValidReporterType)
		if err != nil {
			t.Fatalf("Expected no error creating ReporterType, got %v", err)
		}

		reporterInstanceId, err := NewReporterInstanceId(fixture.ValidReporterInstanceId)
		if err != nil {
			t.Fatalf("Expected no error creating ReporterInstanceId, got %v", err)
		}

		reporter := NewReporterId(reporterType, reporterInstanceId)

		assertValidReporter(t, reporter, nil, fixture.ValidReporterType, fixture.ValidReporterInstanceId)
	})

	t.Run("should create reporter with another valid set of inputs", func(t *testing.T) {
		t.Parallel()

		reporterType, err := NewReporterType(fixture.AnotherReporterType)
		if err != nil {
			t.Fatalf("Expected no error creating ReporterType, got %v", err)
		}

		reporterInstanceId, err := NewReporterInstanceId(fixture.AnotherReporterInstanceId)
		if err != nil {
			t.Fatalf("Expected no error creating ReporterInstanceId, got %v", err)
		}

		reporter := NewReporterId(reporterType, reporterInstanceId)

		assertValidReporter(t, reporter, nil, fixture.AnotherReporterType, fixture.AnotherReporterInstanceId)
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.EmptyString)

		assertInvalidReporter(t, err, "ReporterType cannot be empty")
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.WhitespaceString)

		assertInvalidReporter(t, err, "ReporterType cannot be empty")
	})

	t.Run("should reject empty reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterInstanceId(fixture.EmptyString)

		assertInvalidReporter(t, err, "ReporterInstanceId cannot be empty")
	})

	t.Run("should reject whitespace-only reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterInstanceId(fixture.WhitespaceString)

		assertInvalidReporter(t, err, "ReporterInstanceId cannot be empty")
	})

	t.Run("should reject both empty inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.EmptyString)

		assertInvalidReporter(t, err, "ReporterType cannot be empty")
	})

	t.Run("should reject both whitespace inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterType(fixture.WhitespaceString)

		assertInvalidReporter(t, err, "ReporterType cannot be empty")
	})
}

func assertValidReporter(t *testing.T, reporter ReporterId, err error, expectedType, expectedInstanceId string) {
	t.Helper()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if reporter.reporterType.String() != expectedType {
		t.Errorf("Expected reporter type %s, got %s", expectedType, reporter.reporterType.String())
	}
	if reporter.reporterInstanceId.String() != expectedInstanceId {
		t.Errorf("Expected reporter instance id %s, got %s", expectedInstanceId, reporter.reporterInstanceId.String())
	}
}

func assertInvalidReporter(t *testing.T, err error, expectedErrorSubstring string) {
	t.Helper()
	if err == nil {
		t.Error("Expected error, got none")
	}
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Expected error containing %s, got %v", expectedErrorSubstring, err)
	}
}
