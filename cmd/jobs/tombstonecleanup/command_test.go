package tombstonecleanup

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
)

// TestCommandDefaultsAndValidation protects preview defaults and rejects invalid
// batch controls before attempting a database connection.
func TestCommandDefaultsAndValidation(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	for flag, expected := range map[string]string{
		"execute": "false", "tombstone-days": "30", "rows-per-transaction": "1000",
		"batch-delay": "500ms", "transaction-limit": "0",
	} {
		require.NotNil(t, command.Flags().Lookup(flag), flag)
		require.Equal(t, expected, command.Flags().Lookup(flag).DefValue, flag)
	}
	cases := []struct {
		args    []string
		message string
	}{
		{[]string{"--tombstone-days=-1"}, "tombstone-days"},
		{[]string{"--tombstone-days=0"}, "tombstone-days"},
		{[]string{"--rows-per-transaction=0"}, "rows-per-transaction"},
		{[]string{"--rows-per-transaction=4"}, "rows-per-transaction"},
		{[]string{"--batch-delay=-1s"}, "batch delay"},
		{[]string{"--transaction-limit=-1"}, "transaction limit"},
		{[]string{"--tombstone-days=" + strconv.Itoa(int(^uint(0)>>1))}, "tombstone-days"},
		{[]string{"--rows-per-transaction=65535"}, "rows-per-transaction"},
	}
	if strconv.IntSize == 64 {
		cases = append(cases, struct {
			args    []string
			message string
		}{
			[]string{"--rows-per-transaction=5", "--transaction-limit=" + strconv.Itoa(int(^uint(0)>>1))}, "overflow",
		})
	}
	for _, test := range cases {
		t.Run(test.args[0], func(t *testing.T) {
			command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
			command.SetArgs(test.args)
			require.ErrorContains(t, command.Execute(), test.message, test.args)
		})
	}
}

// TestExecutionRequiresConsumerBoundaryAcknowledgment prevents accidentally
// pruning data that an unverified replication backlog may still need.
func TestExecutionRequiresConsumerBoundaryAcknowledgment(t *testing.T) {
	command := NewCommand(storage.NewOptions(), common.LoggerOptions{})
	command.SetArgs([]string{"--execute"})
	require.ErrorContains(t, command.Execute(), "--execute requires --consumer-boundary-verified")
}

// TestValidationAllowsTechnicalBoundaries keeps defaults and unlimited execution
// available without introducing arbitrary limits on the amount of cleanup work.
func TestValidationAllowsTechnicalBoundaries(t *testing.T) {
	for _, test := range []options{
		{tombstoneDays: 7, rowsPerTransaction: 1000},
		{tombstoneDays: 30, rowsPerTransaction: 65534},
		{tombstoneDays: 30, rowsPerTransaction: 5, transactionLimit: int(^uint(0)>>1) / 5},
	} {
		require.NoError(t, validateInvocation(test))
	}
}

// TestFinalLogging distinguishes process outcome from scoped worklist completion.
func TestFinalLogging(t *testing.T) {
	for _, failure := range []error{nil, context.Canceled} {
		var output bytes.Buffer
		logResult(log.NewHelper(log.NewStdLogger(&output)), options{rowsPerTransaction: 2, transactionLimit: 1}, true, "tombstone cleanup result", result{batches: 1, reporterResources: 2, complete: true}, failure)
		require.Contains(t, output.String(), "worklist_limit=2")
		require.Contains(t, output.String(), "reporter_resources=2")
		if failure == nil {
			require.Contains(t, output.String(), "INFO")
			require.Contains(t, output.String(), "status=succeeded")
		} else {
			require.Contains(t, output.String(), "ERROR")
			require.Contains(t, output.String(), "status=failed")
		}
		require.NotContains(t, output.String(), "cutoff=")
	}
}
