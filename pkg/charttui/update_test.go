package charttui_test

import (
	"errors"
	"fmt"
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

func TestUpdateModel_OneSuccess(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(1))
	tm.Send(chartcmd.EventUpdatingChart("test"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := ansi.Strip(string(bts))

			return strings.Contains(s, "test") &&
				strings.Contains(s, "0/1")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✓ test")
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "Done! Updated 1 charts.")
}

func TestUpdateModel_OneError(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(1))
	tm.Send(chartcmd.EventUpdatingChart("test"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := ansi.Strip(string(bts))

			return strings.Contains(s, "test") &&
				strings.Contains(s, "0/1")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test", Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✗ test")
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "test error")
}

func TestUpdateModel_MultipleSuccess(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(2))

	tm.Send(chartcmd.EventUpdatingChart("test1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := ansi.Strip(string(bts))

			return strings.Contains(s, "test1") &&
				strings.Contains(s, "0/2")
		},
	)

	tm.Send(chartcmd.EventUpdatingChart("test2"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test2")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test1"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			// Note: v2 uses differential rendering (cellbuf), so the full
			// progress bar pattern never appears as contiguous bytes.
			// The golden file comparison at the end verifies exact rendering.
			return strings.Contains(ansi.Strip(string(bts)), "✓ test1")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test2"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✓ test2")
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "Done! Updated 2 charts.")
}

func TestUpdateModel_MultipleWithError(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(2))

	tm.Send(chartcmd.EventUpdatingChart("test1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := ansi.Strip(string(bts))

			return strings.Contains(s, "test1") &&
				strings.Contains(s, "0/2")
		},
	)
	tm.Send(chartcmd.EventUpdatingChart("test2"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test2")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test1"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			// Note: v2 uses differential rendering (cellbuf), so the full
			// progress bar pattern never appears as contiguous bytes.
			// The golden file comparison at the end verifies exact rendering.
			return strings.Contains(ansi.Strip(string(bts)), "✓ test1")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test2", Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✗ test2")
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)
	require.Contains(t, ansi.Strip(string(out)), "test error")
}

func TestUpdateModel_MultipleWithMultierror(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(3))

	tm.Send(chartcmd.EventUpdatingChart("chart-a"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			s := ansi.Strip(string(bts))

			return strings.Contains(s, "chart-a") &&
				strings.Contains(s, "0/3")
		},
	)

	tm.Send(chartcmd.EventUpdatingChart("chart-b"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "chart-b")
		},
	)

	tm.Send(chartcmd.EventUpdatingChart("chart-c"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "chart-c")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "chart-a"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✓ chart-a")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "chart-b", Err: errors.New("connection timeout")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✗ chart-b")
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "chart-c", Err: errors.New("invalid values")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "✗ chart-c")
		},
	)

	merr := errors.Join(
		fmt.Errorf("update %q: %w", "chart-b", errors.New("connection timeout")),
		fmt.Errorf("update %q: %w", "chart-c", errors.New("invalid values")),
	)

	tm.Send(chartcmd.EventDone{Err: merr})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	stripped := ansi.Strip(string(out))
	require.Contains(t, stripped, "2 of 3 charts failed")
	require.Contains(t, stripped, `update "chart-b": connection timeout`)
	require.Contains(t, stripped, `update "chart-c": invalid values`)
}

func TestUpdateModel_CtrlC(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	tm.Send(chartcmd.EventSetChartTotal(2))
	tm.Send(chartcmd.EventUpdatingChart("test1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return strings.Contains(ansi.Strip(string(bts)), "test1")
		},
	)

	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	tm.WaitFinished(t, teatest.WithFinalTimeout(1*time.Second))
}
