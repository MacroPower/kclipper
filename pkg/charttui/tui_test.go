package charttui_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"go.jacobcolvin.com/niceyaml"
	"go.jacobcolvin.com/niceyaml/paths"
	"go.jacobcolvin.com/x/stringtest"

	"github.com/macropower/kclipper/pkg/charttui"
)

// stripTrailingSpaces removes trailing spaces from every line so golden strings
// do not need to account for lipgloss width-padding.
func stripTrailingSpaces(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		lines = append(lines, strings.TrimRight(line, " "))
	}

	return strings.Join(lines, "\n")
}

// testPrinter returns a deterministic [niceyaml.WrappingPrinter] with no
// gutter and no styling, so golden strings remain stable across themes.
func testPrinter() niceyaml.WrappingPrinter {
	return niceyaml.NewPrinter(
		niceyaml.WithGutter(niceyaml.NoGutter()),
		niceyaml.WithStyle(lipgloss.NewStyle()),
	)
}

func TestGetErrorMessage(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input       error
		width       int
		totalCharts []int
		want        string
	}{
		"single plain error": {
			input: errors.New("something went wrong"),
			width: 80,
			want: stringtest.JoinLF(
				"",
				"  ✗ something went wrong",
				"",
				"  1 of 1 charts failed",
				"",
				"",
			),
		},
		"multiple plain errors with totalCharts": {
			input: errors.Join(
				fmt.Errorf("update %q: %w", "chart-a", errors.New("connection timeout")),
				fmt.Errorf("update %q: %w", "chart-b", errors.New("invalid values")),
			),
			width:       80,
			totalCharts: []int{3},
			want: stringtest.JoinLF(
				"",
				`  ✗ update "chart-a": connection timeout`,
				"",
				`  ✗ update "chart-b": invalid values`,
				"",
				"  2 of 3 charts failed",
				"",
				"",
			),
		},
		"single niceyaml annotated error": {
			input: fmt.Errorf("%w", niceyaml.NewError("invalid yaml",
				niceyaml.WithPath(paths.Root().Child("key").Value()),
				niceyaml.WithSource(niceyaml.NewSourceFromString(
					stringtest.Input(`
						key: value
						bad: line
					`)+"\n",
				)),
				niceyaml.WithSourceLines(1),
				niceyaml.WithPrinter(testPrinter()),
			)),
			width: 80,
			want: stringtest.JoinLF(
				"",
				"  ✗ [1:6] invalid yaml:",
				"",
				"    key: value",
				"    bad: line",
				"",
				"  1 of 1 charts failed",
				"",
				"",
			),
		},
		"niceyaml error wrapped in fmt.Errorf chain": {
			input: fmt.Errorf("parse chart: %w", niceyaml.NewError("invalid yaml",
				niceyaml.WithPath(paths.Root().Child("key").Value()),
				niceyaml.WithSource(niceyaml.NewSourceFromString(
					stringtest.Input(`
						key: value
						bad: line
					`)+"\n",
				)),
				niceyaml.WithSourceLines(1),
				niceyaml.WithPrinter(testPrinter()),
			)),
			width: 80,
			want: stringtest.JoinLF(
				"",
				"  ✗ parse chart: [1:6] invalid yaml:",
				"",
				"    key: value",
				"    bad: line",
				"",
				"  1 of 1 charts failed",
				"",
				"",
			),
		},
		"long header with niceyaml annotation at narrow width": {
			input: fmt.Errorf("chart: %w", niceyaml.NewError(
				"this is a very long error message that should be wrapped across multiple lines",
				niceyaml.WithPath(paths.Root().Child("foo").Value()),
				niceyaml.WithSource(niceyaml.NewSourceFromString("foo: bar\n")),
				niceyaml.WithSourceLines(1),
				niceyaml.WithPrinter(testPrinter()),
			)),
			width: 40,
			want: stringtest.JoinLF(
				"",
				"  ✗ chart: [1:6] this is a very long",
				"    error message that should be",
				"    wrapped across multiple lines:",
				"",
				"    foo: bar",
				"",
				"  1 of 1 charts failed",
				"",
				"",
			),
		},
		"mixed joined error with plain and annotated": {
			input: errors.Join(
				errors.New("plain error here"),
				niceyaml.NewError("bad value",
					niceyaml.WithPath(paths.Root().Child("key").Value()),
					niceyaml.WithSource(niceyaml.NewSourceFromString("key: value\n")),
					niceyaml.WithSourceLines(1),
					niceyaml.WithPrinter(testPrinter()),
				),
			),
			width: 80,
			want: stringtest.JoinLF(
				"",
				"  ✗ plain error here",
				"",
				"  ✗ [1:6] bad value:",
				"",
				"    key: value",
				"",
				"  2 of 2 charts failed",
				"",
				"",
			),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := charttui.GetErrorMessage(tc.input, tc.width, tc.totalCharts...)
			stripped := stripTrailingSpaces(ansi.Strip(got))

			assert.Equal(t, tc.want, stripped)
		})
	}
}

func TestGetErrorMessage_Wrapping(t *testing.T) {
	t.Parallel()

	longErr := errors.New(strings.Repeat("x", 200))
	merr := errors.Join(longErr, errors.New("short"))

	got := charttui.GetErrorMessage(merr, 40)
	stripped := ansi.Strip(got)

	// The full error text must be present (wrapped across lines, not truncated).
	xCount := strings.Count(stripped, "x")
	assert.Equal(t, 200, xCount, "all characters should be preserved after wrapping")

	// Every output line must fit within the terminal width.
	for line := range strings.SplitSeq(stripped, "\n") {
		assert.LessOrEqual(t, ansi.StringWidth(line), 40, "line should fit within terminal width: %q", line)
	}
}
