package kclchart

import (
	"regexp"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
)

type (
	ChartBase       kclhelm.ChartBase
	HelmChartConfig kclhelm.ChartConfig
	HelmChart       kclhelm.Chart
)

const (
	helmKCLImport       string = "import helm\n\n"
	helmChartKCLType    string = "helm.Chart"
	repositoriesKCLName string = "repositories"
	repositoriesKCLType string = "[helm.ChartRepo]"
	valuesKCLName       string = "values"
	valuesKCLType       string = "Values | any"
)

var (
	schemaRegexp       = regexp.MustCompile(`schema\s+(\S+):(.*)`)
	valuesRegexp       = regexp.MustCompile(`(\s+` + valuesKCLName + `\??\s*:\s+)any(.*)`)
	repositoriesRegexp = regexp.MustCompile(`(\s+` + repositoriesKCLName + `\??\s*:\s+)any(.*)`)

	genOptInheritHelmChart = jsonschema.Replace(schemaRegexp, helmKCLImport+"schema ${1}("+helmChartKCLType+"):${2}")
	genOptFixValues        = jsonschema.Replace(valuesRegexp, "${1}"+valuesKCLType+"${2}")
	genOptFixChartRepo     = jsonschema.Replace(repositoriesRegexp, "${1}"+repositoriesKCLType+"${2}")
)

//nolint:unparam
func newSchemaReflector() (*jsonschema.Reflector, error) {
	r := jsonschema.NewReflector()

	return r, nil
}
