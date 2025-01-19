package kclhelm

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"

	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

var (
	SchemaDefinitionRegexp   = regexp.MustCompile(`schema\s+(\S+):(.*)`)
	SchemaRepositoriesRegexp = regexp.MustCompile(`(\s+repositories\??\s*:\s+)any(.*)`)
	SchemaPostRendererRegexp = regexp.MustCompile(`(\s+postRenderer\??\s*:\s+)any(.*)`)
)

const (
	PostRendererKCLType string = "({str:}) -> {str:}"
	RepositoriesKCLType string = "[ChartRepo]"
)

// Represents attributes common in `helm.Chart` and `helm.ChartConfig`.
type ChartBase struct {
	// Helm chart name.
	Chart string `json:"chart"`
	// URL of the Helm chart repository.
	RepoURL string `json:"repoURL"`
	// Semver tag for the chart's version.
	TargetRevision string `json:"targetRevision"`
	// Helm release name to use. If omitted the chart name will be used.
	ReleaseName string `json:"releaseName,omitempty"`
	// Optional namespace to template with.
	Namespace string `json:"namespace,omitempty"`
	// Set to `True` to skip the custom resource definition installation step (Helm's `--skip-crds`).
	SkipCRDs bool `json:"skipCRDs,omitempty"`
	// Set to `True` to pass credentials to all domains (Helm's `--pass-credentials`).
	PassCredentials bool `json:"passCredentials,omitempty"`
	// Validator to use for the Values schema.
	SchemaValidator jsonschema.ValidatorType `json:"schemaValidator,omitempty"`
	// Helm chart repositories.
	Repositories []ChartRepo `json:"repositories,omitempty"`
}

func (c *ChartBase) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartBase{}))

	js.SetProperty("schemaValidator", jsonschema.WithEnum(jsonschema.ValidatorTypeEnum))
	js.SetProperty("repositories", jsonschema.WithType("null"), jsonschema.WithNoItems())

	err = js.GenerateKCL(w,
		jsonschema.Replace(SchemaRepositoriesRegexp, "${1}"+RepositoriesKCLType+"${2}"),
	)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

// Defines a Helm chart.
type Chart struct {
	// Helm values to be passed to Helm template. These take precedence over valueFiles.
	Values any `json:"values,omitempty"`
	// Helm value files to be passed to Helm template.
	ValueFiles []string `json:"valueFiles,omitempty"`
	// Lambda function to modify the Helm template output. Evaluated for each resource in the Helm template output.
	PostRenderer any `json:"postRenderer,omitempty"`
}

func (c *Chart) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(Chart{}))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b,
		jsonschema.Replace(SchemaDefinitionRegexp, "schema ${1}(ChartBase):${2}"),
		jsonschema.Replace(SchemaPostRendererRegexp, "${1}"+PostRendererKCLType+"${2}"),
		jsonschema.Replace(SchemaRepositoriesRegexp, "${1}"+RepositoriesKCLType+"${2}"),
	)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}
	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}

// Configuration that can be defined in `charts.k`, in addition to those
// specified in `helm.ChartBase`.
type ChartConfig struct {
	// Schema generator to use for the Values schema.
	SchemaGenerator jsonschema.GeneratorType `json:"schemaGenerator,omitempty"`
	// Path to the schema to use, when relevant for the selected schemaGenerator.
	SchemaPath string `json:"schemaPath,omitempty"`
	// Path to any CRDs to import as schemas. Glob patterns are supported.
	CRDPath string `json:"crdPath,omitempty"`
}

func (c *ChartConfig) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartConfig{}))

	js.SetProperty("schemaGenerator", jsonschema.WithEnum(jsonschema.GeneratorTypeEnum))

	b := &bytes.Buffer{}
	err = js.GenerateKCL(b, jsonschema.Replace(SchemaDefinitionRegexp, "schema ${1}(ChartBase):${2}"))
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	if _, err := b.WriteTo(w); err != nil {
		return fmt.Errorf("failed to write to KCL schema: %w", err)
	}

	return nil
}

// Defines a Helm chart repository.
type ChartRepo struct {
	// Helm chart repository name for reference by `@name`.
	Name string `json:"name"`
	// Helm chart repository URL.
	URL string `json:"url"`

	// Basic authentication username environment variable.
	UsernameEnv string `json:"usernameEnv,omitempty"`
	// Basic authentication password environment variable.
	PasswordEnv string `json:"passwordEnv,omitempty"`

	// CA file path.
	CAPath string `json:"caPath,omitempty"`
	// TLS client certificate data path.
	TLSClientCertDataPath string `json:"tlsClientCertDataPath,omitempty"`
	// TLS client certificate key path.
	TLSClientCertKeyPath string `json:"tlsClientCertKeyPath,omitempty"`

	// Set to `True` to skip SSL certificate verification.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
	// Set to `True` to allow credentials to be used in chart dependencies defined
	// by charts in this repository.
	PassCredentials bool `json:"passCredentials,omitempty"`
}

func (c *ChartRepo) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Name)
}

func (c *ChartRepo) FromMap(m map[string]any) error {
	if name, ok := m["name"].(string); ok {
		c.Name = name
		delete(m, "name")
	}
	if url, ok := m["url"].(string); ok {
		c.URL = url
		delete(m, "url")
	}
	if usernameEnv, ok := m["usernameEnv"].(string); ok {
		c.UsernameEnv = usernameEnv
		delete(m, "usernameEnv")
	}
	if passwordEnv, ok := m["passwordEnv"].(string); ok {
		c.PasswordEnv = passwordEnv
		delete(m, "passwordEnv")
	}
	if caPath, ok := m["caPath"].(string); ok {
		c.CAPath = caPath
		delete(m, "caPath")
	}
	if tlsClientCertDataPath, ok := m["tlsClientCertDataPath"].(string); ok {
		c.TLSClientCertDataPath = tlsClientCertDataPath
		delete(m, "tlsClientCertDataPath")
	}
	if tlsClientCertKeyPath, ok := m["tlsClientCertKeyPath"].(string); ok {
		c.TLSClientCertKeyPath = tlsClientCertKeyPath
		delete(m, "tlsClientCertKeyPath")
	}
	if insecureSkipVerify, ok := m["insecureSkipVerify"].(bool); ok {
		c.InsecureSkipVerify = insecureSkipVerify
		delete(m, "insecureSkipVerify")
	}
	if passCredentials, ok := m["passCredentials"].(bool); ok {
		c.PassCredentials = passCredentials
		delete(m, "passCredentials")
	}
	if len(m) > 0 {
		return fmt.Errorf("unexpected keys in input data: %#v", m)
	}
	return nil
}

func (c *ChartRepo) GetHelmRepo() (*helmrepo.Repo, error) {
	repo := &helmrepo.Repo{
		Name:               c.Name,
		URL:                c.URL,
		CAPath:             c.CAPath,
		InsecureSkipVerify: c.InsecureSkipVerify,
		PassCredentials:    c.PassCredentials,
	}

	if c.UsernameEnv != "" {
		username, ok := os.LookupEnv(c.UsernameEnv)
		if !ok {
			return nil, fmt.Errorf("failed to get username, environment variable '%s' is unset", c.UsernameEnv)
		}
		repo.Username = username
	}
	if c.PasswordEnv != "" {
		password, ok := os.LookupEnv(c.PasswordEnv)
		if !ok {
			return nil, fmt.Errorf("failed to get password, environment variable '%s' is unset", c.PasswordEnv)
		}
		repo.Password = password
	}
	if c.TLSClientCertDataPath != "" {
		tlsClientCertData, err := os.ReadFile(c.TLSClientCertDataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read TLS client certificate data from '%s': %w", c.TLSClientCertDataPath, err)
		}
		repo.TLSClientCertData = tlsClientCertData
	}
	if c.TLSClientCertKeyPath != "" {
		tlsClientCertKey, err := os.ReadFile(c.TLSClientCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read TLS client certificate key from '%s': %w", c.TLSClientCertKeyPath, err)
		}
		repo.TLSClientCertKey = tlsClientCertKey
	}

	return repo, nil
}

func (c *ChartRepo) GenerateKCL(w io.Writer) error {
	r, err := newSchemaReflector()
	if err != nil {
		return fmt.Errorf("failed to create schema reflector: %w", err)
	}
	js := r.Reflect(reflect.TypeOf(ChartRepo{}))

	err = js.GenerateKCL(w)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Schema to KCL Schema: %w", err)
	}

	return nil
}

func newSchemaReflector() (*jsonschema.Reflector, error) {
	r := jsonschema.NewReflector()
	err := r.AddGoComments("github.com/MacroPower/kclipper", "./pkg/kclhelm")
	if err != nil {
		return nil, fmt.Errorf("failed to add go comments: %w", err)
	}

	return r, nil
}
