package chartcmd

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"

	"kcl-lang.io/kcl-go"

	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/kclmodule/kclchart"
)

func (c *KCLPackage) Set(chart, keyValueOverrides string) error {
	if chart == "" {
		return errors.New("chart name cannot be empty")
	}

	logger := slog.With(
		slog.String("cmd", "chart_set"),
		slog.String("chart_key", chart),
	)

	hc := kclchart.Chart{
		ChartBase: kclchart.ChartBase{
			Chart: chart,
		},
	}

	key, value, found := strings.Cut(keyValueOverrides, "=")
	if !found {
		return fmt.Errorf("no key=value pair found in %q", keyValueOverrides)
	}

	configValue := reflect.ValueOf(&kclchart.ChartConfig{}).Elem().FieldByNameFunc(func(fieldName string) bool {
		return strings.EqualFold(fieldName, key)
	})

	if !configValue.CanSet() {
		return fmt.Errorf("key %q is not a valid chart configuration attribute", key)
	}

	setAutomation := kclautomation.Automation{key: kclautomation.NewString(value)}
	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclautomation.SpecPathJoin("charts", hc.GetSnakeCaseName())

	logger.Info("updating charts.k",
		slog.String("spec", chartsSpec),
		slog.String("path", chartsFile),
	)

	err := c.updateFile(setAutomation, chartsFile, initialChartContents, chartsSpec)
	if err != nil {
		return fmt.Errorf("update %q: %w", chartsFile, err)
	}

	logger.Info("formatting kcl files", slog.String("path", c.BasePath))

	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("format kcl files: %w", err)
	}

	return nil
}
