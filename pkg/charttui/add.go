package charttui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/pkg/chartcmd"
)

// AddModel displays the status of an add operation with a spinner, per-item
// results, and a final summary.
type AddModel struct {
	kind string
	name string
	baseModel
}

// NewAddModel creates an [AddModel] used to display the status of adding a
// new item. It renders a spinner which is replaced with a result.
func NewAddModel(kind, name string) *AddModel {
	return &AddModel{
		baseModel: newBaseModel(),
		kind:      kind,
		name:      name,
	}
}

func (m *AddModel) Init() tea.Cmd {
	m.state = stateWorking

	return m.spinner.Tick
}

//nolint:ireturn // Third-party.
func (m *AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case chartcmd.EventAdded:
		icon := defaultStyles.check
		if msg.Err != nil {
			icon = defaultStyles.cross
		}

		return m, tea.Printf("%s %s", icon, m.name)

	default:
		if cmd, handled := m.handleCommon(msg); handled {
			return m, cmd
		}
	}

	return m, nil
}

func (m *AddModel) View() tea.View {
	switch m.state {
	case stateError:
		return tea.NewView(getErrorMessage(m.err, m.width))

	case stateDone:
		return tea.NewView(defaultStyles.done.Render(fmt.Sprintf("Done! Added %s %s.\n", m.kind, m.name)))

	case stateWorking:
		spin := m.spinner.View() + " "
		cellsAvail := max(0, m.width-lipgloss.Width(spin))

		currentName := defaultStyles.itemName.Render(m.name)
		info := lipgloss.NewStyle().MaxWidth(cellsAvail).Render("Adding " + currentName)

		cellsRemaining := max(0, m.width-lipgloss.Width(spin+info))
		gap := strings.Repeat(" ", cellsRemaining) + "\n"

		return tea.NewView(spin + info + gap)

	case stateIdle:
		return tea.NewView("")
	}

	return tea.NewView("")
}
