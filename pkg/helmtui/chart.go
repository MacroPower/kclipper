package helmtui

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
	"github.com/MacroPower/kclipper/pkg/log"
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

func NewChartTUI(w io.Writer, logLevel string, pkg ChartCommander) (*ChartTUI, error) {
	c := &ChartTUI{
		pkg: pkg,
		w:   w,
	}

	c.pkg.Subscribe(c.broadcastEvent)

	logger, err := log.CreateHandler(c, logLevel, log.FormatText)
	if err != nil {
		return nil, fmt.Errorf("failed to create log handler: %w", err)
	}

	slog.SetDefault(slog.New(logger))

	return c, nil
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
	c.p = tea.NewProgram(NewInitModel(), tea.WithOutput(c.w))

	go func() {
		_, err := c.pkg.Init()
		c.broadcastEvent(helmutil.EventDone{Err: err})
	}()

	if _, err := c.p.Run(); err != nil {
		return false, fmt.Errorf("failed to launch tui: %w", err)
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
		c.broadcastEvent(helmutil.EventAdded{Err: err})
		c.broadcastEvent(helmutil.EventDone{Err: err})
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) AddRepo(repo *kclhelm.ChartRepo) error {
	c.p = tea.NewProgram(NewAddModel("repo", repo.Name), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.AddRepo(repo)
		c.broadcastEvent(helmutil.EventAdded{Err: err})
		c.broadcastEvent(helmutil.EventDone{Err: err})
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) Set(chart, keyValueOverrides string) error {
	err := c.pkg.Set(chart, keyValueOverrides)
	if err != nil {
		return fmt.Errorf("failed to set chart arguments: %w", err)
	}

	_, err = fmt.Fprintf(c.w, "Updated %s.\n", chart)
	if err != nil {
		return fmt.Errorf("failed to write to output: %w", err)
	}

	return nil
}

func (c *ChartTUI) Update(charts ...string) error {
	c.p = tea.NewProgram(NewUpdateModel(), tea.WithOutput(c.w))

	go func() {
		err := c.pkg.Update(charts...)
		c.broadcastEvent(helmutil.EventDone{Err: err})
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}
