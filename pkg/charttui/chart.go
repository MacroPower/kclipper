package charttui

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"

	"go.jacobcolvin.com/x/log"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/pkg/chartcmd"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
)

type ChartTUI struct {
	pkg     ChartCommander
	p       atomic.Pointer[tea.Program]
	w       io.Writer
	teaOpts []tea.ProgramOption
}

type ChartCommander interface {
	Init() (bool, error)
	AddChart(key string, chart *kclchart.ChartConfig) error
	AddRepo(repo *kclhelm.ChartRepo) error
	Set(chart, keyValueOverrides string) error
	Update(charts ...string) error
	Subscribe(f func(any))
}

// ChartTUIOption configures a [ChartTUI].
type ChartTUIOption func(*ChartTUI)

// WithProgramOptions appends additional [tea.ProgramOption] values that will be
// applied to every [tea.Program] created by the [ChartTUI]. This is useful for
// testing in non-TTY environments where [tea.WithInput](nil) can be passed to
// disable stdin.
func WithProgramOptions(opts ...tea.ProgramOption) ChartTUIOption {
	return func(c *ChartTUI) {
		c.teaOpts = append(c.teaOpts, opts...)
	}
}

func NewChartTUI(w io.Writer, lvl log.Level, pkg ChartCommander, opts ...ChartTUIOption) *ChartTUI {
	c := &ChartTUI{
		pkg: pkg,
		w:   w,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.pkg.Subscribe(c.broadcastEvent)

	slog.SetDefault(
		slog.New(log.NewHandler(c, lvl, log.FormatText)),
	)

	return c
}

func (c *ChartTUI) newProgram(m tea.Model) *tea.Program {
	opts := make([]tea.ProgramOption, 0, len(c.teaOpts)+1)
	opts = append(opts, tea.WithOutput(c.w))
	opts = append(opts, c.teaOpts...)

	return tea.NewProgram(m, opts...)
}

func (c *ChartTUI) run(m tea.Model, work func()) error {
	p := c.newProgram(m)

	c.p.Store(p)
	defer c.p.Store(nil)

	go work()

	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) broadcastEvent(evt any) {
	if p := c.p.Load(); p != nil {
		p.Send(evt)
	}
}

func (c *ChartTUI) Write(p []byte) (int, error) {
	c.broadcastEvent(teaMsgWriteLog(string(p)))

	return len(p), nil
}

func (c *ChartTUI) Subscribe(f func(any)) {
	c.pkg.Subscribe(f)
}

func (c *ChartTUI) Init() (bool, error) {
	err := c.run(NewActionModel("initialization", "initializing"), func() {
		_, err := c.pkg.Init()
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *ChartTUI) AddChart(key string, chart *kclchart.ChartConfig) error {
	if key == "" {
		return errors.New("chart key is required")
	}

	return c.run(NewAddModel("chart", key), func() {
		err := c.pkg.AddChart(key, chart)
		c.broadcastEvent(chartcmd.EventAdded{Err: err})
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	})
}

func (c *ChartTUI) AddRepo(repo *kclhelm.ChartRepo) error {
	return c.run(NewAddModel("repo", repo.Name), func() {
		err := c.pkg.AddRepo(repo)
		c.broadcastEvent(chartcmd.EventAdded{Err: err})
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	})
}

func (c *ChartTUI) Set(chart, keyValueOverrides string) error {
	return c.run(NewActionModel("update", "updating"), func() {
		err := c.pkg.Set(chart, keyValueOverrides)
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	})
}

func (c *ChartTUI) Update(charts ...string) error {
	return c.run(NewUpdateModel(), func() {
		err := c.pkg.Update(charts...)
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	})
}
