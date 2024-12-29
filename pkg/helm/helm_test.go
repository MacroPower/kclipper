package helm_test

import (
	"testing"

	"github.com/MacroPower/kclx/pkg/helm"
	"github.com/MacroPower/kclx/pkg/jsonschema"
)

// func TestGetHelmValuesSchema(t *testing.T) {
// 	t.Parallel()

// 	yamls, err := helm.DefaultHelm.GetValuesSchemas(&helm.TemplateOpts{
// 		ChartName:      "podinfo",
// 		TargetRevision: "6.7.1",
// 		RepoURL:        "https://stefanprodan.github.io/podinfo",
// 	})
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	_, okDefault := yamls["values.yaml"]
// 	if !okDefault {
// 		t.Fatalf("values.yaml not found")
// 	}
// 	_, okProd := yamls["values-prod.yaml"]
// 	if !okProd {
// 		t.Fatalf("values-prod.yaml not found")
// 	}
// }

func TestGetHelmValuesJsonSchema(t *testing.T) {
	t.Parallel()

	jsonSchema, err := helm.DefaultHelm.GetValuesJSONSchema(&helm.TemplateOpts{
		ChartName:      "podinfo",
		TargetRevision: "6.7.1",
		RepoURL:        "https://stefanprodan.github.io/podinfo",
	}, jsonschema.DefaultAutoGenerator)
	if err != nil {
		t.Fatal(err)
	}

	if jsonSchema == nil {
		t.Fatalf("json schema not found")
	}
}
