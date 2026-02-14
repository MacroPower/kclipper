package charttui_test

import (
	"errors"
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"

	"github.com/macropower/kclipper/pkg/charttui"
)

func TestActionModelView(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		model *charttui.ActionModel
	}{
		"initial": {
			model: charttui.NewTestActionModel(
				"Initialization", "Initializing", 0, charttui.StateIdle, nil,
			),
		},
		"working": {
			model: charttui.NewTestActionModel(
				"Initialization", "Initializing", 80, charttui.StateWorking, nil,
			),
		},
		"done_success": {
			model: charttui.NewTestActionModel(
				"Initialization", "Initializing", 0, charttui.StateDone, nil,
			),
		},
		"done_error": {
			model: charttui.NewTestActionModel(
				"Initialization", "Initializing", 80, charttui.StateError,
				errors.New("something went wrong"),
			),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.model.View()
			golden.RequireEqual(t, got.Content)
		})
	}
}

func TestAddModelView(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		model *charttui.AddModel
	}{
		"initial": {
			model: charttui.NewTestAddModel(
				"chart", "my-chart", 0, charttui.StateIdle, nil,
			),
		},
		"working": {
			model: charttui.NewTestAddModel(
				"chart", "my-chart", 80, charttui.StateWorking, nil,
			),
		},
		"done_success": {
			model: charttui.NewTestAddModel(
				"chart", "my-chart", 0, charttui.StateDone, nil,
			),
		},
		"done_error": {
			model: charttui.NewTestAddModel(
				"chart", "my-chart", 80, charttui.StateError,
				errors.New("chart not found"),
			),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.model.View()
			golden.RequireEqual(t, got.Content)
		})
	}
}

func TestUpdateModelView(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		model *charttui.UpdateModel
	}{
		"initial": {
			model: charttui.NewTestUpdateModel(
				[]string{}, []string{},
				0, 0, charttui.StateIdle, nil,
			),
		},
		"working_one_chart": {
			model: charttui.NewTestUpdateModel(
				[]string{"ingress-nginx"}, []string{},
				3, 80, charttui.StateWorking, nil,
			),
		},
		"working_multiple_charts": {
			model: charttui.NewTestUpdateModel(
				[]string{"ingress-nginx", "cert-manager", "prometheus"},
				[]string{"ingress-nginx"},
				3, 80, charttui.StateWorking, nil,
			),
		},
		"done_success": {
			model: charttui.NewTestUpdateModel(
				[]string{"ingress-nginx", "cert-manager"},
				[]string{"ingress-nginx", "cert-manager"},
				2, 0, charttui.StateDone, nil,
			),
		},
		"done_error": {
			model: charttui.NewTestUpdateModel(
				[]string{"ingress-nginx"},
				[]string{"ingress-nginx"},
				1, 80, charttui.StateError,
				errors.New("update failed: connection refused"),
			),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := tc.model.View()
			golden.RequireEqual(t, got.Content)
		})
	}
}

func TestActionModelView_EmptyWhenNotStarted(t *testing.T) {
	t.Parallel()

	m := charttui.NewTestActionModel("Test", "Testing", 0, charttui.StateIdle, nil)
	got := m.View()
	assert.Empty(t, got.Content)
}

func TestAddModelView_EmptyWhenNotStarted(t *testing.T) {
	t.Parallel()

	m := charttui.NewTestAddModel("chart", "test", 0, charttui.StateIdle, nil)
	got := m.View()
	assert.Empty(t, got.Content)
}

func TestUpdateModelView_EmptyWhenNotStarted(t *testing.T) {
	t.Parallel()

	m := charttui.NewTestUpdateModel(
		[]string{}, []string{},
		0, 0, charttui.StateIdle, nil,
	)
	got := m.View()
	assert.Empty(t, got.Content)
}
