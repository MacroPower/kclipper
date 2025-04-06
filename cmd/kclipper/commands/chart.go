package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/MacroPower/kclipper/pkg/crd"
	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmtui"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
	"github.com/MacroPower/kclipper/pkg/log"
)

const (
	chartDesc = `This command manages kcl charts
`
	chartExample = `  kcl chart <command> [arguments]...
  # Initialize the current module
  kcl chart init

  # Add chart for the current module
  kcl chart add --chart podinfo --repo_url https://stefanprodan.github.io/podinfo --target_revision 6.7.0

  # Update all chart schemas for the current module
  kcl chart update

  # Update a specific chart's schemas for the current module
  kcl chart update --chart podinfo

  # Set chart configuration attributes
  kcl chart set --chart podinfo --overrides "targetRevision=6.7.1"
`
)

var (
	ErrArgument           = errors.New("argument error")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrChartCommandFailed = errors.New("chart command failed")
	ErrChartInitFailed    = errors.New("chart init failed")
	ErrChartAddFailed     = errors.New("chart add failed")
	ErrChartUpdateFailed  = errors.New("chart update failed")
	ErrChartSetFailed     = errors.New("chart set failed")
	ErrChartRepoAddFailed = errors.New("chart repo add failed")
)

// NewChartCmd returns the chart command.
func NewChartCmd(arg *RootArgs) *cobra.Command {
	args := NewChartArgs(arg)

	cmd := &cobra.Command{
		Use:          "chart",
		Short:        "Helm chart management",
		Long:         chartDesc,
		Example:      chartExample,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(args.path, "path", "p", "charts", "Base path for the charts package")
	if err := cmd.MarkPersistentFlagDirname("path"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().DurationVar(args.timeout, "timeout", 5*time.Minute, "Timeout for the command")
	cmd.PersistentFlags().BoolVarP(args.quiet, "quiet", "q", false, "Run in quiet mode")
	cmd.PersistentFlags().BoolVarP(args.vendor, "vendor", "V", false, "Run in vendor mode")
	cmd.PersistentFlags().StringVar(args.maxExtractSize, "max_extract_size", "10Mi", "Maximum size of extracted charts")

	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if _, err := resource.ParseQuantity(*args.maxExtractSize); err != nil {
			return fmt.Errorf("%w: %w: max_extract_size: %w", ErrArgument, ErrInvalidArgument, err)
		}

		return nil
	}

	cmd.AddCommand(NewChartInitCmd(args))
	cmd.AddCommand(NewChartAddCmd(args))
	cmd.AddCommand(NewChartUpdateCmd(args))
	cmd.AddCommand(NewChartSetCmd(args))
	cmd.AddCommand(NewChartRepoCmd(args))

	return cmd
}

func NewChartInitCmd(args *ChartArgs) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the current module",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := newChartCommander(cmd.OutOrStdout(), args)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartCommandFailed, err)
			}

			_, err = cc.Init()
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartInitFailed, err)
			}

			return nil
		},
		SilenceUsage: true,
	}
}

func NewChartAddCmd(args *ChartArgs) *cobra.Command {
	chart := new(string)
	repoURL := new(string)
	targetRevision := new(string)
	schemaGenerator := new(string)
	schemaValidator := new(string)
	schemaPath := new(string)
	crdGenerator := new(string)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new chart",
		RunE: func(cmd *cobra.Command, _ []string) error {
			schemaGeneratorType := jsonschema.GetGeneratorType(*schemaGenerator)
			schemaValidatorType := jsonschema.GetValidatorType(*schemaValidator)
			crdGeneratorType := crd.GetGeneratorType(*crdGenerator)

			cc, err := newChartCommander(cmd.OutOrStdout(), args)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartCommandFailed, err)
			}

			cConfig := &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:           *chart,
					RepoURL:         *repoURL,
					TargetRevision:  *targetRevision,
					SchemaValidator: schemaValidatorType,
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: schemaGeneratorType,
					SchemaPath:      *schemaPath,
					CRDGenerator:    crdGeneratorType,
				},
			}

			err = cc.AddChart(cConfig.GetSnakeCaseName(), cConfig)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartAddFailed, err)
			}

			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(chart, "chart", "c", "", "Helm chart name (required)")
	cmd.Flags().StringVarP(repoURL, "repo_url", "r", "", "URL of the Helm chart repository (required)")
	cmd.Flags().StringVarP(targetRevision, "target_revision", "t", "", "Semver tag for the chart's version")
	cmd.Flags().StringVar(schemaGenerator, "schema_generator", "", "Chart schema generator")
	cmd.Flags().StringVar(schemaValidator, "schema_validator", "", "Chart schema validator")
	cmd.Flags().StringVar(schemaPath, "schema_path", "", "Chart schema path")
	cmd.Flags().StringVar(crdGenerator, "crd_generator", "", "CRD generator")

	must(cmd.MarkFlagRequired("chart"))
	must(cmd.MarkFlagRequired("repo_url"))

	return cmd
}

func NewChartUpdateCmd(args *ChartArgs) *cobra.Command {
	charts := new([]string)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update charts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := newChartCommander(cmd.OutOrStdout(), args)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartCommandFailed, err)
			}

			err = cc.Update(*charts...)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartUpdateFailed, err)
			}

			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringSliceVarP(charts, "chart", "c", []string{}, "Helm chart to update (if unset, updates all charts)")

	return cmd
}

func NewChartSetCmd(args *ChartArgs) *cobra.Command {
	chart := new(string)
	overrides := new(string)

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set chart configuration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := newChartCommander(cmd.OutOrStdout(), args)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartCommandFailed, err)
			}

			err = cc.Set(*chart, *overrides)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartSetFailed, err)
			}

			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(chart, "chart", "c", "", "Specify the Helm chart name (required)")
	cmd.Flags().StringVarP(overrides, "overrides", "O", "", "Specify the configuration override path and value (required)")

	must(cmd.MarkFlagRequired("chart"))
	must(cmd.MarkFlagRequired("overrides"))

	return cmd
}

func NewChartRepoCmd(args *ChartArgs) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Helm chart repository management",
	}
	cmd.AddCommand(NewChartRepoAddCmd(args))

	return cmd
}

func NewChartRepoAddCmd(args *ChartArgs) *cobra.Command {
	name := new(string)
	url := new(string)
	usernameEnv := new(string)
	passwordEnv := new(string)
	caPath := new(string)
	tlsClientCertDataPath := new(string)
	tlsClientCertKeyPath := new(string)
	insecureSkipVerify := new(bool)
	passCredentials := new(bool)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new chart repository",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cc, err := newChartCommander(cmd.OutOrStdout(), args)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartCommandFailed, err)
			}

			cr := &kclhelm.ChartRepo{
				Name:                  *name,
				URL:                   *url,
				UsernameEnv:           *usernameEnv,
				PasswordEnv:           *passwordEnv,
				CAPath:                *caPath,
				TLSClientCertDataPath: *tlsClientCertDataPath,
				TLSClientCertKeyPath:  *tlsClientCertKeyPath,
				InsecureSkipVerify:    *insecureSkipVerify,
				PassCredentials:       *passCredentials,
			}

			err = cc.AddRepo(cr)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrChartRepoAddFailed, err)
			}

			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().StringVarP(name, "name", "n", "", "Helm chart repository name (required)")
	cmd.Flags().StringVarP(url, "url", "u", "", "URL of the Helm chart repository (required)")
	cmd.Flags().StringVarP(usernameEnv, "username_env", "U", "", "Basic authentication username environment variable")
	cmd.Flags().StringVarP(passwordEnv, "password_env", "P", "", "Basic authentication password environment variable")
	cmd.Flags().StringVar(caPath, "ca_path", "", "CA file path")
	cmd.Flags().StringVar(tlsClientCertDataPath, "tls_client_cert_data_path", "", "TLS client certificate data path")
	cmd.Flags().StringVar(tlsClientCertKeyPath, "tls_client_cert_key_path", "", "TLS client certificate key path")
	cmd.Flags().BoolVar(insecureSkipVerify, "insecure_skip_verify", false, "Skip SSL certificate verification")
	cmd.Flags().BoolVar(passCredentials, "pass_credentials", false, "Pass credentials to the Helm chart repository")

	must(cmd.MarkFlagRequired("name"))
	must(cmd.MarkFlagRequired("url"))

	return cmd
}

type chartCommander interface {
	Init() (bool, error)
	AddChart(key string, chart *kclchart.ChartConfig) error
	AddRepo(repo *kclhelm.ChartRepo) error
	Set(chart, keyValueOverrides string) error
	Update(charts ...string) error
	Subscribe(f func(any))
}

//nolint:ireturn // Multiple concrete types.
func newChartCommander(w io.Writer, args *ChartArgs) (chartCommander, error) {
	cc, err := helmutil.NewChartPkg(args.GetPath(), helm.DefaultClient,
		helmutil.WithTimeout(args.GetTimeout()),
		helmutil.WithVendor(args.GetVendor()),
		helmutil.WithMaxExtractSize(args.GetMaxExtractSize()),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrChartInitFailed, err)
	}

	if args.GetQuiet() || !isatty.IsTerminal(os.Stdout.Fd()) {
		return cc, nil
	}

	lvl, err := log.GetLevel(args.GetLogLevel())
	if err != nil {
		// Should not be possible due to root's PersistentPreRunE.
		return nil, fmt.Errorf("%w: %w", ErrArgument, err)
	}

	return helmtui.NewChartTUI(w, lvl, cc), nil
}

type ChartArgs struct {
	path           *string
	maxExtractSize *string
	timeout        *time.Duration
	quiet          *bool
	vendor         *bool
	*RootArgs
}

func NewChartArgs(args *RootArgs) *ChartArgs {
	return &ChartArgs{
		path:           new(string),
		maxExtractSize: new(string),
		timeout:        new(time.Duration),
		quiet:          new(bool),
		vendor:         new(bool),
		RootArgs:       args,
	}
}

func (a *ChartArgs) GetPath() string {
	return *a.path
}

func (a *ChartArgs) GetMaxExtractSize() *resource.Quantity {
	size, err := resource.ParseQuantity(*a.maxExtractSize)
	if err != nil {
		panic(err)
	}

	return &size
}

func (a *ChartArgs) GetTimeout() time.Duration {
	return *a.timeout
}

func (a *ChartArgs) GetQuiet() bool {
	return *a.quiet
}

func (a *ChartArgs) GetVendor() bool {
	return *a.vendor
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
