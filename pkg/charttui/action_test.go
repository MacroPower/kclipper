package charttui_test

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/stretchr/testify/require"

	"github.com/MacroPower/kclipper/pkg/chartcmd"
	"github.com/MacroPower/kclipper/pkg/charttui"
)

func TestActionModel_Success(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Initializing"))
		},
	)

	tm.Send(chartcmd.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestActionModel_Error(t *testing.T) {
	t.Parallel()

	m := charttui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Initializing"))
		},
	)

	tm.Send(chartcmd.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}
