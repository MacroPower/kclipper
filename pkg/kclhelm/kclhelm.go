package kclhelm

import (
	"fmt"
	"regexp"

	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

const (
	chartBaseKCLType    string = "ChartBase"
	repositoriesKCLName string = "repositories"
	repositoriesKCLType string = "[ChartRepo]"
	postRendererKCLName string = "postRenderer"
	postRendererKCLType string = "(Resource) -> Resource"
)

var (
	schemaDefinitionRegexp = regexp.MustCompile(`schema\s+(\S+):(.*)`)
	repositoriesRegexp     = regexp.MustCompile(`(\s+` + repositoriesKCLName + `\??\s*:\s+)any(.*)`)
	postRendererRegexp     = regexp.MustCompile(`(\s+` + postRendererKCLName + `\??\s*:\s+)any(.*)`)

	genOptInheritChartBase = jsonschema.Replace(schemaDefinitionRegexp, "schema ${1}("+chartBaseKCLType+"):${2}")
	genOptFixChartRepo     = jsonschema.Replace(repositoriesRegexp, "${1}"+repositoriesKCLType+"${2}")
	genOptFixPostRenderer  = jsonschema.Replace(postRendererRegexp, "${1}"+postRendererKCLType+"${2}")
)

func newSchemaReflector() (*jsonschema.Reflector, error) {
	r := jsonschema.NewReflector()

	err := r.AddGoComments("github.com/MacroPower/kclipper", "./pkg/kclhelm")
	if err != nil {
		return nil, fmt.Errorf("failed to add go comments: %w", err)
	}

	return r, nil
}
