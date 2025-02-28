package helmtui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	currentNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	doneStyle        = lipgloss.NewStyle().Margin(1, 2)
	errStyle         = lipgloss.NewStyle().Margin(1, 2)
	progressStyle    = lipgloss.NewStyle().Margin(1, 2)
	spinnerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))
	checkMark        = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).SetString("✓")
	errorMark        = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).SetString("✗")
)

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

func keyExits(msg tea.KeyMsg) bool {
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

func getErrorMessage(err error, width int) string {
	errMsg := fmt.Sprintf("%v", err)
	errMsg = strings.Trim(errMsg, "\r\n")

	return errStyle.Width(max(0, width-2)).Render(errMsg + "\n")
}
