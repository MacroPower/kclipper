package charttui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/hashicorp/go-multierror"

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

type (
	// Sent to write a log message.
	teaMsgWriteLog string
)

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

func writeLog(msg teaMsgWriteLog, width int) tea.Cmd {
	logMsg := string(msg)
	logMsg = strings.Trim(logMsg, "\r\n")
	logMsg = lipgloss.NewStyle().Width(max(0, width-2)).Render(logMsg)

	return tea.Println(logMsg)
}

func getErrorMessage(err error, width int, totalCharts ...int) string {
	var merr *multierror.Error
	if !errors.As(err, &merr) || len(merr.Errors) <= 1 {
		errMsg := fmt.Sprintf("%v", err)
		errMsg = strings.Trim(errMsg, "\r\n")

		return defaultStyles.err.Width(max(0, width-2)).Render(errMsg + "\n")
	}

	maxWidth := max(0, width-2)
	lines := make([]string, 0, len(merr.Errors)+1)

	for _, e := range merr.Errors {
		line := fmt.Sprintf("%s %s", defaultStyles.cross, e)
		line = lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
		lines = append(lines, line)
	}

	failedCount := len(merr.Errors)
	total := failedCount
	if len(totalCharts) > 0 && totalCharts[0] > 0 {
		total = totalCharts[0]
	}

	summary := fmt.Sprintf("%d of %d charts failed", failedCount, total)
	lines = append(lines, summary)

	return defaultStyles.err.Render(strings.Join(lines, "\n") + "\n")
}
