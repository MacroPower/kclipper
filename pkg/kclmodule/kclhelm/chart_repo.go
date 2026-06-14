package kclhelm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/iancoleman/strcase"

	"github.com/macropower/kclipper/pkg/helmrepo"
	"github.com/macropower/kclipper/pkg/kclautomation"
	"github.com/macropower/kclipper/pkg/schema"
)

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

func (c *ChartRepo) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}

	if c.URL == "" {
		return errors.New("url is required")
	}

	return nil
}

func (c *ChartRepo) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Name)
}

func (c *ChartRepo) GenerateKCL(w io.Writer) error {
	js, err := schema.Reflect[ChartRepo](schema.WithGoComments())
	if err != nil {
		return fmt.Errorf("reflect schema: %w", err)
	}

	err = js.GenerateKCL(w)
	if err != nil {
		return fmt.Errorf("convert JSON Schema to KCL schema: %w", err)
	}

	return nil
}

// FromMap populates the [ChartRepo] from a decoded KCL map, rejecting any keys
// that do not correspond to a field. The map's values keep their native Go
// types (string/bool), so a JSON round-trip reproduces the struct exactly.
func (c *ChartRepo) FromMap(m map[string]any) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal repository: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	err = dec.Decode(c)
	if err != nil {
		return fmt.Errorf("decode repository: %w", err)
	}

	return nil
}

func (c *ChartRepo) GetHelmRepo() (*helmrepo.RepoOpts, error) {
	repo := &helmrepo.RepoOpts{
		Name:                  c.Name,
		URL:                   c.URL,
		CAPath:                c.CAPath,
		TLSClientCertDataPath: c.TLSClientCertDataPath,
		TLSClientCertKeyPath:  c.TLSClientCertKeyPath,
		InsecureSkipVerify:    c.InsecureSkipVerify,
		PassCredentials:       c.PassCredentials,
	}

	if c.UsernameEnv != "" {
		username, ok := os.LookupEnv(c.UsernameEnv)
		if !ok {
			return nil, fmt.Errorf("failed to get username, environment variable %q is unset", c.UsernameEnv)
		}

		repo.Username = username
	}

	if c.PasswordEnv != "" {
		password, ok := os.LookupEnv(c.PasswordEnv)
		if !ok {
			return nil, fmt.Errorf("failed to get password, environment variable %q is unset", c.PasswordEnv)
		}

		repo.Password = password
	}

	return repo, nil
}

func (c *ChartRepo) ToAutomation() kclautomation.Automation {
	return kclautomation.Automation{
		"name":                  kclautomation.NewString(c.Name),
		"url":                   kclautomation.NewString(c.URL),
		"usernameEnv":           kclautomation.NewString(c.UsernameEnv),
		"passwordEnv":           kclautomation.NewString(c.PasswordEnv),
		"caPath":                kclautomation.NewString(c.CAPath),
		"tlsClientCertDataPath": kclautomation.NewString(c.TLSClientCertDataPath),
		"tlsClientCertKeyPath":  kclautomation.NewString(c.TLSClientCertKeyPath),
		"insecureSkipVerify":    kclautomation.NewBool(c.InsecureSkipVerify),
		"passCredentials":       kclautomation.NewBool(c.PassCredentials),
	}
}
