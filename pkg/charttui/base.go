package charttui

import (
	"time"

	"charm.land/bubbles/v2/spinner"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/pkg/chartcmd"
)

// modelState represents the lifecycle state of a TUI model.
type modelState int

const (
	stateIdle    modelState = iota // Initial state before work begins.
	stateWorking                   // Work is in progress.
	stateDone                      // Work completed successfully.
	stateError                     // Work completed with an error.
)

// completedMsg signals that the pre-quit delay has elapsed and the model
// should transition to its final state. This is the second phase of the
// two-phase quit sequence, ensuring all pending messages are rendered
// before the program exits.
type completedMsg struct {
	err error
}

// baseModel contains shared fields and behavior for all charttui models.
// Models embed this struct and delegate common message handling to
// [baseModel.handleCommon].
type baseModel struct {
	err     error
	spinner spinner.Model
	width   int
	state   modelState
}

// newBaseModel creates a [baseModel] with a default spinner.
func newBaseModel() baseModel {
	s := spinner.New()
	s.Style = defaultStyles.spinner

	return baseModel{
		spinner: s,
	}
}

// handleCommon processes messages shared across all models.
// It returns a command and a boolean indicating whether the message was
// handled. If handled is true, the caller should return the command
// immediately without further processing.
func (b *baseModel) handleCommon(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		b.width = msg.Width

		return nil, true

	case tea.KeyPressMsg:
		if keyExits(msg) {
			return tea.Quit, true
		}

	case teaMsgWriteLog:
		return writeLog(msg, b.width), true

	case chartcmd.EventDone:
		return tea.Sequence(
			tea.Tick(preQuitDelay, func(_ time.Time) tea.Msg {
				return completedMsg{err: msg.Err}
			}),
			teaQuit(),
		), true

	case completedMsg:
		if msg.err != nil {
			b.state = stateError
			b.err = msg.err
		} else {
			b.state = stateDone
		}

		return nil, true

	case spinner.TickMsg:
		var cmd tea.Cmd

		b.spinner, cmd = b.spinner.Update(msg)

		return cmd, true
	}

	return nil, false
}
