package helmutil

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"kcl-lang.io/kcl-go"

	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclutil"
)

func (c *ChartPkg) Set(chart, keyValueOverrides string) error {
	if chart == "" {
		return errors.New("chart name cannot be empty")
	}

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

	setAutomation := kclutil.Automation{key: kclutil.NewString(value)}
	chartsFile := filepath.Join(c.BasePath, "charts.k")
	chartsSpec := kclutil.SpecPathJoin("charts", hc.GetSnakeCaseName())

	err := c.updateFile(setAutomation, chartsFile, initialChartContents, chartsSpec)
	if err != nil {
		return fmt.Errorf("failed to update %q: %w", chartsFile, err)
	}

	_, err = kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}
