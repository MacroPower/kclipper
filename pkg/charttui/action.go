package charttui

import (
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kclipper/pkg/chartcmd"
)

type ActionModel struct {
	err     error
	noun    string
	verb    string
	spinner spinner.Model
	width   int
	height  int
	mu      sync.RWMutex
	working bool
	done    bool
}

// NewActionModel creates an [ActionModel] used to display the status of a
// simple action. It renders a spinner which is replaced with a result. If any
// logs are written, they will be displayed in the terminal above the spinner.
// `noun`: the outcome or instance of the action (e.g., "update").
// `verb`: the ongoing action using present participle tense (e.g., "updating").
func NewActionModel(noun, verb string) *ActionModel {
	s := spinner.New()
	s.Style = spinnerStyle

	caser := cases.Title(language.English)

	return &ActionModel{
		noun:    caser.String(noun),
		verb:    caser.String(verb),
		spinner: s,
		mu:      sync.RWMutex{},
	}
}

func (m *ActionModel) Init() tea.Cmd {
	m.working = true

	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *ActionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if keyExits(msg) {
			return m, tea.Quit
		}

	case teaMsgWriteLog:
		return m, writeLog(msg, m.width)

	case chartcmd.EventDone:
		m.working = false

		// Allow previously sent messages to be drawn.
		preQuitCmd := tea.Tick(time.Millisecond*100, func(_ time.Time) tea.Msg {
			m.mu.Lock()
			defer m.mu.Unlock()

			m.err = msg.Err
			m.done = true

			return nil
		})

		return m, tea.Sequence(preQuitCmd, teaQuit())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m *ActionModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return getErrorMessage(m.err, m.width)
	}

	if m.done {
		return doneStyle.Render(m.noun + " complete.\n")
	}

	if m.working {
		spin := m.spinner.View() + " "
		cellsAvail := max(0, m.width-lipgloss.Width(spin))

		info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render(m.verb)

		cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
		gap := strings.Repeat(" ", cellsRemaining) + "\n"

		return spin + info + gap
	}

	return ""
}
