package charttui_test

import (
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.jacobcolvin.com/x/log"

	"github.com/macropower/kclipper/pkg/charttui"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
)

// mockChartCommander is a mock implementation of [charttui.ChartCommander]
// for testing the [charttui.ChartTUI] orchestrator.
type mockChartCommander struct {
	mu          sync.Mutex
	subscribers []func(any)

	initCalled bool
	addCalled  bool
	setCalled  bool

	initResult bool
	initErr    error
	addErr     error
	setErr     error
}

func (m *mockChartCommander) Init() (bool, error) {
	m.mu.Lock()

	m.initCalled = true
	m.mu.Unlock()

	return m.initResult, m.initErr
}

func (m *mockChartCommander) AddChart(_ string, _ *kclchart.ChartConfig) error {
	m.mu.Lock()

	m.addCalled = true
	m.mu.Unlock()

	return m.addErr
}

func (m *mockChartCommander) AddRepo(_ *kclhelm.ChartRepo) error {
	return nil
}

func (m *mockChartCommander) Set(_, _ string) error {
	m.mu.Lock()

	m.setCalled = true
	m.mu.Unlock()

	return m.setErr
}

func (m *mockChartCommander) Update(_ ...string) error {
	return nil
}

func (m *mockChartCommander) Subscribe(f func(any)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.subscribers = append(m.subscribers, f)
}

func TestChartTUI_AddChart_EmptyKey(t *testing.T) {
	t.Parallel()

	mock := &mockChartCommander{}
	tui := charttui.NewChartTUI(io.Discard, log.LevelInfo, mock)

	err := tui.AddChart("", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chart key is required")
	assert.False(t, mock.addCalled, "AddChart should not be called with empty key")
}

func TestChartTUI_Init(t *testing.T) {
	t.Parallel()

	mock := &mockChartCommander{initResult: true}
	tui := charttui.NewChartTUI(io.Discard, log.LevelInfo, mock)

	ok, err := tui.Init()
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, mock.initCalled, "Init should be called on the underlying commander")
}

func TestChartTUI_Init_Error(t *testing.T) {
	t.Parallel()

	mock := &mockChartCommander{
		initResult: false,
		initErr:    errors.New("init broken"),
	}
	tui := charttui.NewChartTUI(io.Discard, log.LevelInfo, mock)

	// TUI itself should not return error; the inner error is displayed in the TUI.
	_, err := tui.Init()
	require.NoError(t, err)
	assert.True(t, mock.initCalled)
}

func TestChartTUI_Set(t *testing.T) {
	t.Parallel()

	mock := &mockChartCommander{}
	tui := charttui.NewChartTUI(io.Discard, log.LevelInfo, mock)

	err := tui.Set("my-chart", "key=value")
	require.NoError(t, err)
	assert.True(t, mock.setCalled, "Set should be called on the underlying commander")
}

func TestChartTUI_Set_Error(t *testing.T) {
	t.Parallel()

	mock := &mockChartCommander{setErr: errors.New("set broken")}
	tui := charttui.NewChartTUI(io.Discard, log.LevelInfo, mock)

	// TUI itself should not return error; the inner error is displayed in the TUI.
	err := tui.Set("my-chart", "key=value")
	require.NoError(t, err)
	assert.True(t, mock.setCalled)
}
