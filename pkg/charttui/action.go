package charttui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	tea "charm.land/bubbletea/v2"
)

// ActionModel displays the status of a simple action with a spinner that is
// replaced with a result.
type ActionModel struct {
	noun string
	verb string
	baseModel
}

// NewActionModel creates an [ActionModel] used to display the status of a
// simple action. It renders a spinner which is replaced with a result. If any
// logs are written, they will be displayed in the terminal above the spinner.
// `noun`: the outcome or instance of the action (e.g., "update").
// `verb`: the ongoing action using present participle tense (e.g., "updating").
func NewActionModel(noun, verb string) *ActionModel {
	caser := cases.Title(language.English)

	return &ActionModel{
		baseModel: newBaseModel(),
		noun:      caser.String(noun),
		verb:      caser.String(verb),
	}
}

func (m *ActionModel) Init() tea.Cmd {
	m.state = stateWorking

	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *ActionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.handleCommon(msg); handled {
		return m, cmd
	}

	return m, nil
}

func (m *ActionModel) View() tea.View {
	switch m.state {
	case stateError:
		return tea.NewView(getErrorMessage(m.err, m.width))

	case stateDone:
		return tea.NewView(defaultStyles.done.Render(m.noun + " complete.\n"))

	case stateWorking:
		spin := m.spinner.View() + " "
		cellsAvail := max(0, m.width-lipgloss.Width(spin))

		info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render(m.verb)

		cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
		gap := strings.Repeat(" ", cellsRemaining) + "\n"

		return tea.NewView(spin + info + gap)

	case stateIdle:
		return tea.NewView("")
	}

	return tea.NewView("")
}
