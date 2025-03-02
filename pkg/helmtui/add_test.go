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

func TestAddModel_Success(t *testing.T) {
	t.Parallel()

	m := helmtui.NewAddModel("chart", "test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test"))
		},
	)

	tm.Send(helmutil.EventAdded{})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✓ test"))
		},
	)

	tm.Send(helmutil.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestAddModel_Error(t *testing.T) {
	t.Parallel()

	m := helmtui.NewAddModel("chart", "test")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("test"))
		},
	)

	tm.Send(helmutil.EventAdded{Err: errors.New("test error")})
	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("✗ test"))
		},
	)

	tm.Send(helmutil.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}
