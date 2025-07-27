package charttui_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/pkg/chartcmd"
	"github.com/macropower/kclipper/pkg/charttui"
)

func TestUpdateModel_OneSuccess(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	time.Sleep(100 * time.Millisecond)

	tm.Send(chartcmd.EventSetChartTotal(1))
	tm.Send(chartcmd.EventUpdatingChart("test"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test")) &&
				bytes.Contains(bts, []byte("░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 0/1"))
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✓ test"))
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestUpdateModel_OneError(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	time.Sleep(100 * time.Millisecond)

	tm.Send(chartcmd.EventSetChartTotal(1))
	tm.Send(chartcmd.EventUpdatingChart("test"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test")) &&
				bytes.Contains(bts, []byte("░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 0/1"))
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test", Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✗ test"))
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestUpdateModel_MultipleSuccess(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	time.Sleep(100 * time.Millisecond)

	tm.Send(chartcmd.EventSetChartTotal(2))

	tm.Send(chartcmd.EventUpdatingChart("test1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test1")) &&
				bytes.Contains(bts, []byte("░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 0/2"))
		},
	)

	tm.Send(chartcmd.EventUpdatingChart("test2"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test2"))
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test1"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✓ test1")) &&
				bytes.Contains(bts, []byte("████████████████████░░░░░░░░░░░░░░░░░░░░ 1/2"))
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test2"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✓ test2"))
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestUpdateModel_MultipleWithError(t *testing.T) {
	t.Parallel()

	m := charttui.NewUpdateModel()
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	time.Sleep(100 * time.Millisecond)

	tm.Send(chartcmd.EventSetChartTotal(2))

	tm.Send(chartcmd.EventUpdatingChart("test1"))
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test1")) &&
				bytes.Contains(bts, []byte("░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░ 0/2"))
		},
	)
	tm.Send(chartcmd.EventUpdatingChart("test2"))

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test1"})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✓ test1")) &&
				bytes.Contains(bts, []byte("████████████████████░░░░░░░░░░░░░░░░░░░░ 1/2"))
		},
	)

	tm.Send(chartcmd.EventUpdatedChart{Chart: "test2", Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✗ test2"))
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}
