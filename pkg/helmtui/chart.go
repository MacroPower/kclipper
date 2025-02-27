package helmtui

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/log"
)

type ChartTUI struct {
	pkg *helmutil.ChartPkg
	p   *tea.Program
}

func NewChartTUI(pkg *helmutil.ChartPkg, logLevel string) (*ChartTUI, error) {
	c := &ChartTUI{
		pkg: pkg,
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

func (c *ChartTUI) AddChart(key string, chart *kclchart.ChartConfig) error {
	c.p = tea.NewProgram(NewAddChartModel(key))

	go func() {
		err := c.pkg.AddChart(key, chart)
		c.broadcastEvent(helmutil.EventAddedChart{Err: err})

		if err != nil {
			c.broadcastEvent(fmt.Errorf("%w: %w", helmutil.ErrChartUpdateFailed, err))
		}
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}

func (c *ChartTUI) Update(charts ...string) error {
	c.p = tea.NewProgram(NewUpdateModel())

	go func() {
		err := c.pkg.Update(charts...)
		if err != nil {
			c.broadcastEvent(fmt.Errorf("%w: %w", helmutil.ErrChartUpdateFailed, err))
		}
	}()

	if _, err := c.p.Run(); err != nil {
		return fmt.Errorf("failed to launch tui: %w", err)
	}

	return nil
}
