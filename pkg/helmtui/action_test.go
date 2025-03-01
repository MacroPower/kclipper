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

func TestActionModel_Success(t *testing.T) {
	t.Parallel()

	m := helmtui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)
	time.Sleep(100 * time.Millisecond)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Initializing"))
		},
	)

	tm.Send(helmutil.EventDone{})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}

func TestActionModel_Error(t *testing.T) {
	t.Parallel()

	m := helmtui.NewActionModel("initialization", "initializing")
	tm := teatest.NewTestModel(
		t, m,
		teatest.WithInitialTermSize(300, 100),
	)
	time.Sleep(100 * time.Millisecond)

	teatest.WaitFor(
		t, tm.Output(),
		func(bts []byte) bool {
			return bytes.Contains(bts, []byte("Initializing"))
		},
	)

	tm.Send(helmutil.EventDone{Err: errors.New("test error")})

	out, err := io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(1*time.Second)))
	require.NoError(t, err)

	teatest.RequireEqualOutput(t, out)
}
