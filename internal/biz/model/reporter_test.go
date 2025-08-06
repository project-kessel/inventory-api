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

		reporter, err := NewReporterId(fixture.ValidReporterType, fixture.ValidReporterInstanceId)

		assertValidReporter(t, reporter, err, fixture.ValidReporterType, fixture.ValidReporterInstanceId)
	})

	t.Run("should create reporter with another valid set of inputs", func(t *testing.T) {
		t.Parallel()

		reporter, err := NewReporterId(fixture.AnotherReporterType, fixture.AnotherReporterInstanceId)

		assertValidReporter(t, reporter, err, fixture.AnotherReporterType, fixture.AnotherReporterInstanceId)
	})

	t.Run("should reject empty reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.EmptyString, fixture.ValidReporterInstanceId)

		assertInvalidReporter(t, err, "ReporterId invalid type")
	})

	t.Run("should reject whitespace-only reporter type", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.WhitespaceString, fixture.ValidReporterInstanceId)

		assertInvalidReporter(t, err, "ReporterId invalid type")
	})

	t.Run("should reject empty reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.ValidReporterType, fixture.EmptyString)

		assertInvalidReporter(t, err, "ReporterId invalid instance ID")
	})

	t.Run("should reject whitespace-only reporter instance id", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.ValidReporterType, fixture.WhitespaceString)

		assertInvalidReporter(t, err, "ReporterId invalid instance ID")
	})

	t.Run("should reject both empty inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.EmptyString, fixture.EmptyString)

		assertInvalidReporter(t, err, "ReporterId invalid type")
	})

	t.Run("should reject both whitespace inputs", func(t *testing.T) {
		t.Parallel()

		_, err := NewReporterId(fixture.WhitespaceString, fixture.WhitespaceString)

		assertInvalidReporter(t, err, "ReporterId invalid type")
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
