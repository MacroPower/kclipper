package charttui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"go.jacobcolvin.com/niceyaml"

	tea "charm.land/bubbletea/v2"
)

type styles struct {
	spinner  lipgloss.Style
	done     lipgloss.Style
	err      lipgloss.Style
	progress lipgloss.Style
	itemName lipgloss.Style
	check    lipgloss.Style
	cross    lipgloss.Style
}

var defaultStyles = styles{
	spinner:  lipgloss.NewStyle().Foreground(lipgloss.Color("63")),
	done:     lipgloss.NewStyle().Margin(1, 2),
	err:      lipgloss.NewStyle().Margin(1, 2),
	progress: lipgloss.NewStyle().Margin(1, 2),
	itemName: lipgloss.NewStyle().Foreground(lipgloss.Color("211")),
	check:    lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓"),
	cross:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗"),
}

// preQuitDelay is the time to wait after work completes before quitting,
// allowing previously sent messages to be rendered.
const preQuitDelay = 100 * time.Millisecond

// TeaMsgWriteLog is sent to write a log message to the terminal.
type TeaMsgWriteLog string

func teaQuit() tea.Cmd {
	return tea.Sequence(
		tea.Tick(time.Millisecond*500, func(_ time.Time) tea.Msg {
			return nil
		}),
		tea.Quit,
	)
}

func keyExits(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "ctrl+c", "esc", "q":
		return true
	}

	return false
}

func writeLog(msg TeaMsgWriteLog) tea.Cmd {
	logMsg := string(msg)
	logMsg = strings.Trim(logMsg, "\r\n")

	return tea.Println(logMsg)
}

// crossWidth is the display width of the cross marker plus its trailing space.
const crossWidth = 2

// errStyleMargin is the total horizontal margin of [styles.err] (left + right).
// Must equal the sum of horizontal Margin values in defaultStyles.err above.
const errStyleMargin = 4

// GetErrorMessage formats an error for display in the TUI. For multi-errors,
// it renders each sub-error with a cross marker and appends a failure summary.
func GetErrorMessage(err error, width int, totalCharts ...int) string {
	errs := unwrapErrs(err)

	lines := make([]string, 0, len(errs)+1)
	for i, e := range errs {
		if i > 0 {
			lines = append(lines, "")
		}

		limit := 0
		if l := width - errStyleMargin - crossWidth; l > 0 {
			limit = l
		}

		// Let niceyaml handle its own width-aware formatting.
		var yamlErr *niceyaml.Error

		hasAnnotation := errors.As(e, &yamlErr)
		if hasAnnotation {
			yamlErr.SetWidth(limit)
		}

		errStr := strings.Trim(e.Error(), "\r\n")

		if limit > 0 {
			errStr = wrapErrorString(errStr, limit, hasAnnotation)
		}

		// Keep the first line as-is; drop blank continuation lines and
		// indent the rest to align with text after the cross marker.
		errLines := strings.Split(errStr, "\n")

		filtered := errLines[:1:1]
		for _, el := range errLines[1:] {
			// Preserve blank lines for annotated errors (they separate
			// the header from the source context); drop them for plain errors.
			if !hasAnnotation && strings.TrimSpace(el) == "" {
				continue
			}

			filtered = append(filtered, "  "+el)
		}

		line := fmt.Sprintf("%s %s", defaultStyles.cross, strings.Join(filtered, "\n"))
		lines = append(lines, line)
	}

	failedCount := len(errs)
	total := failedCount
	if len(totalCharts) > 0 && totalCharts[0] > 0 {
		total = totalCharts[0]
	}

	summary := fmt.Sprintf("\n%d of %d charts failed", failedCount, total)
	lines = append(lines, summary)

	return defaultStyles.err.Render(strings.Join(lines, "\n") + "\n")
}

// wrapErrorString word-wraps errStr to fit within limit columns. For
// annotated errors the header (before the first blank-line separator) is
// wrapped while the source annotation is left untouched.
func wrapErrorString(errStr string, limit int, hasAnnotation bool) string {
	if !hasAnnotation {
		return ansi.Wrap(errStr, limit, "")
	}

	sep := strings.Index(errStr, "\n\n")
	if sep < 0 {
		return ansi.Wrap(errStr, limit, "")
	}

	return ansi.Wrap(errStr[:sep], limit, "") + errStr[sep:]
}

func unwrapErrs(err error) []error {
	type unwrapper interface {
		Unwrap() []error
	}

	merr, ok := err.(unwrapper)
	if ok {
		return merr.Unwrap()
	}

	return []error{err}
}
