package charttui_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/internal/teatest"
	"github.com/macropower/kclipper/pkg/chartcmd"
	"github.com/macropower/kclipper/pkg/charttui"
)

func TestAddModel_Success(t *testing.T) {
	t.Parallel()

	m := charttui.NewAddModel("chart", "test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test")
		},
	)

	tm.Send(chartcmd.EventAdded{})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✓ test")
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "Done! Added chart test.")
}

func TestAddModel_Error(t *testing.T) {
	t.Parallel()

	m := charttui.NewAddModel("chart", "test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test")
		},
	)

	tm.Send(chartcmd.EventAdded{Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✗ test")
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "test error")
}

func TestAddModel_CtrlC(t *testing.T) {
	t.Parallel()

	m := charttui.NewAddModel("chart", "test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test")
		},
	)

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	tm.WaitFinished(t, teatest.WithFinalTimeout(1*time.Second))
}
