package charttui

import (
	"charm.land/bubbles/v2/progress"
)

// GetErrorMessage is an exported alias of [getErrorMessage] for testing.
var GetErrorMessage = getErrorMessage

// TeaMsgWriteLog is an alias for [teaMsgWriteLog] exported for testing.
type TeaMsgWriteLog = teaMsgWriteLog

// NewTestActionModel creates an [ActionModel] in a specific state for testing.
func NewTestActionModel(noun, verb string, width int, state modelState, err error) *ActionModel {
	b := newBaseModel()
	b.width = width
	b.state = state
	b.err = err

	return &ActionModel{
		noun:      noun,
		verb:      verb,
		baseModel: b,
	}
}

// NewTestAddModel creates an [AddModel] in a specific state for testing.
func NewTestAddModel(kind, name string, width int, state modelState, err error) *AddModel {
	b := newBaseModel()
	b.width = width
	b.state = state
	b.err = err

	return &AddModel{
		kind:      kind,
		name:      name,
		baseModel: b,
	}
}

// NewTestUpdateModel creates an [UpdateModel] in a specific state for testing.
func NewTestUpdateModel(
	startedCharts, completedCharts []string,
	totalCharts, width int,
	state modelState,
	err error,
) *UpdateModel {
	p := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	b := newBaseModel()
	b.width = width
	b.state = state
	b.err = err

	return &UpdateModel{
		progress:        p,
		startedCharts:   startedCharts,
		completedCharts: completedCharts,
		totalCharts:     totalCharts,
		baseModel:       b,
	}
}

// Exported state constants for testing.
const (
	StateIdle    = stateIdle
	StateWorking = stateWorking
	StateDone    = stateDone
	StateError   = stateError
)
