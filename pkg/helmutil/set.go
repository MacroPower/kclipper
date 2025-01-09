package helmutil

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"kcl-lang.io/kcl-go"

	helmmodels "github.com/MacroPower/kclipper/pkg/helmmodels/chartmodule"
)

func (c *ChartPkg) Set(chart string, keyValueOverrides string) error {
	if chart == "" {
		return errors.New("chart name cannot be empty")
	}

	hc := helmmodels.Chart{
		ChartBase: helmmodels.ChartBase{
			Chart: chart,
		},
	}

	key, value, found := strings.Cut(keyValueOverrides, "=")
	if !found {
		return fmt.Errorf("no key=value pair found in '%s'", keyValueOverrides)
	}

	configValue := reflect.ValueOf(&helmmodels.ChartConfig{}).Elem().FieldByNameFunc(func(fieldName string) bool {
		return strings.EqualFold(fieldName, key)
	})

	if !configValue.CanSet() {
		return fmt.Errorf("key '%s' is not a valid chart configuration attribute", key)
	}

	chartConfig := map[string]string{key: value}
	if err := c.updateChartsFile(c.BasePath, hc.GetSnakeCaseName(), chartConfig); err != nil {
		return err
	}

	_, err := kcl.FormatPath(c.BasePath)
	if err != nil {
		return fmt.Errorf("failed to format kcl files: %w", err)
	}

	return nil
}
