package kclhelm

import (
	"regexp"

	"github.com/macropower/kclipper/pkg/schema"
)

const (
	chartBaseKCLType      string = "ChartBase"
	repositoriesKCLName   string = "repositories"
	repositoriesKCLType   string = "[ChartRepo]"
	valueInferenceKCLName string = "valueInference"
	valueInferenceKCLType string = "ValueInferenceConfig"
	postRendererKCLName   string = "postRenderer"
	postRendererKCLType   string = "(Resource) -> Resource"
)

var (
	schemaDefinitionRegexp = regexp.MustCompile(`schema\s+(\S+):(.*)`)
	repositoriesRegexp     = regexp.MustCompile(`(\s+` + repositoriesKCLName + `\??\s*:\s+)any(.*)`)
	valueInferenceRegexp   = regexp.MustCompile(`(\s+` + valueInferenceKCLName + `\??\s*:\s+)any(.*)`)
	postRendererRegexp     = regexp.MustCompile(`(\s+` + postRendererKCLName + `\??\s*:\s+)any(.*)`)

	genOptInheritChartBase  = schema.Replace(schemaDefinitionRegexp, "schema ${1}("+chartBaseKCLType+"):${2}")
	genOptFixChartRepo      = schema.Replace(repositoriesRegexp, "${1}"+repositoriesKCLType+"${2}")
	genOptFixValueInference = schema.Replace(valueInferenceRegexp, "${1}"+valueInferenceKCLType+"${2}")
	genOptFixPostRenderer   = schema.Replace(postRendererRegexp, "${1}"+postRendererKCLType+"${2}")
)
