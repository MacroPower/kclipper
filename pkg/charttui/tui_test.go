package charttui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/charttui"
)

func TestGetErrorMessage(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input       error
		width       int
		totalCharts []int
		wantLines   []string
		wantAbsent  []string
	}{
		"single error unchanged": {
			input:     errors.New("something went wrong"),
			width:     80,
			wantLines: []string{"something went wrong"},
		},
		"multierror with one sub-error uses single format": {
			input: multierror.Append(nil, errors.New("only one")),
			width: 80,
			wantLines: []string{
				"1 error occurred:",
				"only one",
			},
		},
		"multierror with multiple sub-errors": {
			input: multierror.Append(
				multierror.Append(nil,
					fmt.Errorf("update %q: %w", "chart-a", errors.New("connection timeout")),
				),
				fmt.Errorf("update %q: %w", "chart-b", errors.New("invalid values")),
			),
			width:       80,
			totalCharts: []int{3},
			wantLines: []string{
				`update "chart-a": connection timeout`,
				`update "chart-b": invalid values`,
				"2 of 3 charts failed",
			},
		},
		"multierror without totalCharts uses error count": {
			input: multierror.Append(
				multierror.Append(nil,
					errors.New("err1"),
				),
				errors.New("err2"),
			),
			width: 80,
			wantLines: []string{
				"err1",
				"err2",
				"2 of 2 charts failed",
			},
		},
		"multierror lines contain error mark": {
			input: multierror.Append(
				multierror.Append(nil,
					errors.New("err1"),
				),
				errors.New("err2"),
			),
			width: 300,
			wantLines: []string{
				"✗ err1",
				"✗ err2",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := charttui.GetErrorMessage(tc.input, tc.width, tc.totalCharts...)
			stripped := ansi.Strip(got)

			for _, want := range tc.wantLines {
				assert.Contains(t, stripped, want)
			}

			for _, absent := range tc.wantAbsent {
				assert.NotContains(t, stripped, absent)
			}
		})
	}
}

func TestGetErrorMessage_Truncation(t *testing.T) {
	t.Parallel()

	longErr := errors.New(strings.Repeat("x", 200))
	merr := multierror.Append(
		multierror.Append(nil, longErr),
		errors.New("short"),
	)

	got := charttui.GetErrorMessage(merr, 40)
	stripped := ansi.Strip(got)

	lines := strings.Split(strings.TrimSpace(stripped), "\n")
	require.GreaterOrEqual(t, len(lines), 2)

	// The long error line should be truncated (not contain full 200 chars).
	for _, line := range lines {
		if strings.Contains(line, "xxx") {
			assert.LessOrEqual(t, len(line), 42, "line should be truncated to near terminal width")
		}
	}
}
