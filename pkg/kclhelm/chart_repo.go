package kclhelm

import (
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/iancoleman/strcase"

	"github.com/MacroPower/kclipper/pkg/helmrepo"
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

func (c *ChartRepo) GetSnakeCaseName() string {
	return strcase.ToSnake(c.Name)
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
