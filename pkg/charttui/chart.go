package charttui

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/macropower/kclipper/pkg/chartcmd"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
	"github.com/macropower/kclipper/pkg/kclmodule/kclhelm"
	"github.com/macropower/kclipper/pkg/log"
)

type ChartTUI struct {
	pkg ChartCommander
	p   *tea.Program
	w   io.Writer
}

type ChartCommander interface {
	Init() (bool, error)
	AddChart(key string, chart *kclchart.ChartConfig) error
	AddRepo(repo *kclhelm.ChartRepo) error
	Set(chart, keyValueOverrides string) error
	Update(charts ...string) error
	Subscribe(f func(any))
}

func NewChartTUI(w io.Writer, lvl slog.Level, pkg ChartCommander) *ChartTUI {
	c := &ChartTUI{
		pkg: pkg,
		w:   w,
	}

	c.pkg.Subscribe(c.broadcastEvent)

	slog.SetDefault(
		slog.New(log.CreateHandler(c, lvl, log.FormatText)),
	)

	return c
}

func (c *ChartTUI) broadcastEvent(evt any) {
	if c.p != nil {
		c.p.Send(evt)
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
	c.p = tea.NewProgram(NewActionModel("initialization", "initializing"), tea.WithOutput(c.w))

	go func() {
		_, err := c.pkg.Init()
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	}()

	_, err := c.p.Run()
	if err != nil {
		return false, fmt.Errorf("launch tui: %w", err)
	}

	return true, nil
}

func (c *ChartTUI) AddChart(key string, chart *kclchart.ChartConfig) error {
	if key == "" {
		return errors.New("chart key is required")
	}

	c.p = tea.NewProgram(NewAddModel("chart", key), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.AddChart(key, chart)
		c.broadcastEvent(chartcmd.EventAdded{Err: err})
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	}()

	_, err := c.p.Run()
	if err != nil {
		return fmt.Errorf("launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) AddRepo(repo *kclhelm.ChartRepo) error {
	c.p = tea.NewProgram(NewAddModel("repo", repo.Name), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.AddRepo(repo)
		c.broadcastEvent(chartcmd.EventAdded{Err: err})
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	}()

	_, err := c.p.Run()
	if err != nil {
		return fmt.Errorf("launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) Set(chart, keyValueOverrides string) error {
	c.p = tea.NewProgram(NewActionModel("update", "updating"), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.Set(chart, keyValueOverrides)
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	}()

	_, err := c.p.Run()
	if err != nil {
		return fmt.Errorf("launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) Update(charts ...string) error {
	c.p = tea.NewProgram(NewUpdateModel(), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.Update(charts...)
		c.broadcastEvent(chartcmd.EventDone{Err: err})
	}()

	_, err := c.p.Run()
	if err != nil {
		return fmt.Errorf("launch tui: %w", err)
	}

	return nil
}
