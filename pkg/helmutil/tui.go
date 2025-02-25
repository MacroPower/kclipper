package helmutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	currentChartNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle             = lipgloss.NewStyle().Margin(1, 2)
	errStyle              = lipgloss.NewStyle().Margin(1, 2)
	progressStyle         = lipgloss.NewStyle().Margin(1, 2)
	spinnerStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	checkMark             = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	errorMark             = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗")
)

type (
	// Sent to write a log message.
	teaMsgWriteLog string

	// Sent when a chart has been added.
	teaMsgAddedChart struct{}

	// Sent to update the total chart count.
	teaMsgSetChartTotal int

	// Sent when a chart update has started.
	teaMsgUpdatingChart string

	// Sent when a chart has been updated, or when a fatal error occurs during an update.
	teaMsgUpdatedChart struct {
		err   error
		chart string
	}
)

func finalPause() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(_ time.Time) tea.Msg {
		return nil
	})
}

func writeLog(msg teaMsgWriteLog, width int) tea.Cmd {
	logMsg := string(msg)
	logMsg = strings.Trim(logMsg, "\r\n")
	logMsg = lipgloss.NewStyle().Width(max(0, width-2)).Render(logMsg)

	return tea.Println(logMsg)
}

func getErrorMessage(err error, width int) string {
	errMsg := fmt.Sprintf("%v", err)
	errMsg = strings.Trim(errMsg, "\r\n")

	return errStyle.Width(max(0, width-2)).Render(errMsg + "\n")
}
