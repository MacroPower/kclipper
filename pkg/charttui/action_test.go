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

func TestActionModel_Success(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "Initializing")
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "Initialization complete.")
}

func TestActionModel_Error(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "Initializing")
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "test error")
}

func TestActionModel_WriteLog(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("build", "building")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "Building")
		},
	)

	tm.Send(charttui.TeaMsgWriteLog("processing item 1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "processing item 1")
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "Build complete.")
}

func TestActionModel_CtrlC(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "Initializing")
		},
	)

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	tm.WaitFinished(t, teatest.WithFinalTimeout(1*time.Second))
}
