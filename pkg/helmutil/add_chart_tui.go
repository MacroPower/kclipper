package helmutil

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

type addTUI struct {
	err     error
	chart   string
	spinner spinner.Model
	width   int
	height  int
	done    bool
}

func newAddTUI(chart string) *addTUI {
	s := spinner.New()
	s.Style = spinnerStyle

	return &addTUI{
		spinner: s,
		chart:   chart,
	}
}

func (m *addTUI) Init() tea.Cmd {
	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *addTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			return m, tea.Quit
		}

	case teaMsgWriteLog:
		return m, writeLog(msg, m.width)

	case teaMsgAddedChart:
		m.done = true

		return m, tea.Sequence(
			tea.Printf("%s %s", checkMark, m.chart),
			finalPause(),
			tea.Quit,
		)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case error:
		m.err = msg

		return m, tea.Sequence(
			tea.Printf("%s %s", errorMark, m.chart),
			finalPause(),
			tea.Quit,
		)
	}

	return m, nil
}

func (m *addTUI) View() string {
	if m.err != nil {
		return getErrorMessage(m.err, m.width)
	}

	if m.done {
		return doneStyle.Render("Done! Added 1 chart.\n")
	}

	spin := m.spinner.View() + " "
	cellsAvail := max(0, m.width-lipgloss.Width(spin))

	chartName := currentChartNameStyle.Render(m.chart)
	info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Adding " + chartName)

	cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
	gap := strings.Repeat(" ", cellsRemaining)

	return spin + info + gap + "\n"
}
