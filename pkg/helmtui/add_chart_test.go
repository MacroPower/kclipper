package helmtui_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/helmtui"
	"github.com/MacroPower/kclipper/pkg/helmutil"
)

func TestAddChartModel_Success(t *testing.T) {
	t.Parallel()

	m := helmtui.NewAddChartModel("test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)
	time.Sleep(100 * time.Millisecond)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test"))
		},
	)

	tm.Send(helmutil.EventAddedChart{})

	_, err := io.ReadAll(tm.Output())
	require.NoError(t, err)

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestAddChartModel_Error(t *testing.T) {
	t.Parallel()

	m := helmtui.NewAddChartModel("test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)
	time.Sleep(100 * time.Millisecond)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test"))
		},
	)

	tm.Send(helmutil.EventAddedChart{Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✗ test"))
		},
	)

	tm.Send(errors.New("test error"))

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(10*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}
