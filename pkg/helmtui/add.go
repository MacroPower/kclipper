package helmtui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kclipper/pkg/helmutil"
)

type AddModel struct {
	err     error
	kind    string
	name    string
	spinner spinner.Model
	width   int
	height  int
	mu      sync.RWMutex
	working bool
	done    bool
}

func NewAddModel(kind, name string) *AddModel {
	s := spinner.New()
	s.Style = spinnerStyle

	return &AddModel{
		spinner: s,
		kind:    kind,
		name:    name,
		mu:      sync.RWMutex{},
	}
}

func (m *AddModel) Init() tea.Cmd {
	m.working = true

	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyMsg:
		if keyExits(msg) {
			return m, tea.Quit
		}

	case teaMsgWriteLog:
		return m, writeLog(msg, m.width)

	case helmutil.EventAdded:
		m.working = false

		icon := checkMark
		if msg.Err != nil {
			icon = errorMark
		}

		return m, tea.Printf("%s %s", icon, m.name)

	case helmutil.EventDone:
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

func (m *AddModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.err != nil {
		return getErrorMessage(m.err, m.width)
	}

	if m.done {
		return doneStyle.Render(fmt.Sprintf("Done! Added %s %s.\n", m.kind, m.name))
	}

	if m.working {
		spin := m.spinner.View() + " "
		cellsAvail := max(0, m.width-lipgloss.Width(spin))

		currentName := currentNameStyle.Render(m.name)
		info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Adding " + currentName)

		cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
		gap := strings.Repeat(" ", cellsRemaining) + "\n"

		return spin + info + gap
	}

	return ""
}
