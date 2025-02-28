package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmtui"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
	"github.com/MacroPower/kclipper/pkg/kclchart"
	"github.com/MacroPower/kclipper/pkg/kclhelm"
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

var ErrInvalidArgument = errors.New("invalid argument")

// NewChartCmd returns the chart command.
func NewChartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "chart",
		Short:        "Helm chart management",
		Long:         chartDesc,
		Example:      chartExample,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringP("path", "p", "charts", "Base path for the charts package")
	if err := cmd.MarkPersistentFlagDirname("path"); err != nil {
		panic(err)
	}

	cmd.PersistentFlags().Duration("timeout", 5*time.Minute, "Timeout for the command")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "Run in quiet mode")

	cmd.AddCommand(NewChartInitCmd())
	cmd.AddCommand(NewChartAddCmd())
	cmd.AddCommand(NewChartUpdateCmd())
	cmd.AddCommand(NewChartSetCmd())
	cmd.AddCommand(NewChartRepoCmd())

	return cmd
}

func NewChartInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the current module",
		RunE: func(cc *cobra.Command, _ []string) error {
			var merr error

			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			logLevel, err := flags.GetString("log_level")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			quiet, err := flags.GetBool("quiet")
			if err != nil {
				merr = multierror.Append(merr, err)
			}

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)

			if quiet || !isatty.IsTerminal(os.Stdout.Fd()) {
				_, err := c.Init()
				if err != nil {
					return fmt.Errorf("init failed: %w", err)
				}

				return nil
			}

			ct, err := helmtui.NewChartTUI(c, logLevel)
			if err != nil {
				return fmt.Errorf("failed to create tui: %w", err)
			}

			_, err = ct.Init()
			if err != nil {
				return fmt.Errorf("init failed: %w", err)
			}

			return nil
		},
		SilenceUsage: true,
	}
}

func NewChartAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new chart",
		RunE: func(cc *cobra.Command, _ []string) error {
			var merr error
			if err := cc.MarkFlagRequired("chart"); err != nil {
				merr = multierror.Append(merr, err)
			}
			if err := cc.MarkFlagRequired("repo_url"); err != nil {
				merr = multierror.Append(merr, err)
			}

			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			timeout, err := flags.GetDuration("timeout")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			chart, err := flags.GetString("chart")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			repoURL, err := flags.GetString("repo_url")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			targetRevision, err := flags.GetString("target_revision")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			schemaGeneratorString, err := flags.GetString("schema_generator")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			schemaGenerator := jsonschema.GetGeneratorType(schemaGeneratorString)
			schemaValidatorString, err := flags.GetString("schema_validator")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			schemaValidator := jsonschema.GetValidatorType(schemaValidatorString)
			schemaPath, err := flags.GetString("schema_path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			crdPath, err := flags.GetString("crd_path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			vendor, err := flags.GetBool("vendor")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			logLevel, err := flags.GetString("log_level")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			quiet, err := flags.GetBool("quiet")
			if err != nil {
				merr = multierror.Append(merr, err)
			}

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient,
				helmutil.WithVendor(vendor),
				helmutil.WithTimeout(timeout),
			)

			cConfig := &kclchart.ChartConfig{
				ChartBase: kclchart.ChartBase{
					Chart:           chart,
					RepoURL:         repoURL,
					TargetRevision:  targetRevision,
					SchemaValidator: schemaValidator,
				},
				HelmChartConfig: kclchart.HelmChartConfig{
					SchemaGenerator: schemaGenerator,
					SchemaPath:      schemaPath,
					CRDPath:         crdPath,
				},
			}

			if quiet || !isatty.IsTerminal(os.Stdout.Fd()) {
				return c.AddChart(cConfig.GetSnakeCaseName(), cConfig)
			}

			ct, err := helmtui.NewChartTUI(c, logLevel)
			if err != nil {
				return fmt.Errorf("failed to create tui: %w", err)
			}

			return ct.AddChart(cConfig.GetSnakeCaseName(), cConfig)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringP("chart", "c", "", "Helm chart name (required)")
	cmd.Flags().StringP("repo_url", "r", "", "URL of the Helm chart repository (required)")
	cmd.Flags().StringP("target_revision", "t", "", "Semver tag for the chart's version")
	cmd.Flags().String("schema_generator", "", "Chart schema generator")
	cmd.Flags().String("schema_validator", "KCL", "Chart schema validator")
	cmd.Flags().String("schema_path", "", "Chart schema path")
	cmd.Flags().String("crd_path", "", "CRD path")
	cmd.Flags().BoolP("vendor", "V", false, "Run in vendor mode")

	must(cmd.MarkFlagRequired("chart"))
	must(cmd.MarkFlagRequired("repo_url"))

	return cmd
}

func NewChartUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update charts",
		RunE: func(cc *cobra.Command, _ []string) error {
			var merr error

			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			timeout, err := flags.GetDuration("timeout")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			charts, err := flags.GetStringSlice("chart")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			vendor, err := flags.GetBool("vendor")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			logLevel, err := flags.GetString("log_level")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			quiet, err := flags.GetBool("quiet")
			if err != nil {
				merr = multierror.Append(merr, err)
			}

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient,
				helmutil.WithVendor(vendor),
				helmutil.WithTimeout(timeout),
			)

			if quiet || !isatty.IsTerminal(os.Stdout.Fd()) {
				return c.Update(charts...)
			}

			ct, err := helmtui.NewChartTUI(c, logLevel)
			if err != nil {
				return fmt.Errorf("failed to create tui: %w", err)
			}

			return ct.Update(charts...)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringSliceP("chart", "c", []string{}, "Helm chart to update (if unset, updates all charts)")
	cmd.Flags().BoolP("vendor", "V", false, "Run in vendor mode")

	return cmd
}

func NewChartSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set chart configuration",
		RunE: func(cc *cobra.Command, _ []string) error {
			var merr error

			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			chart, err := flags.GetString("chart")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			overrides, err := flags.GetString("overrides")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			logLevel, err := flags.GetString("log_level")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			quiet, err := flags.GetBool("quiet")
			if err != nil {
				merr = multierror.Append(merr, err)
			}

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)

			if quiet || !isatty.IsTerminal(os.Stdout.Fd()) {
				return c.Set(chart, overrides)
			}

			ct, err := helmtui.NewChartTUI(c, logLevel)
			if err != nil {
				return fmt.Errorf("failed to create tui: %w", err)
			}

			return ct.Set(chart, overrides)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringP("chart", "c", "", "Specify the Helm chart name (required)")
	cmd.Flags().StringP("overrides", "O", "", "Specify the configuration override path and value (required)")

	must(cmd.MarkFlagRequired("chart"))
	must(cmd.MarkFlagRequired("overrides"))

	return cmd
}

func NewChartRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Helm chart repository management",
	}
	cmd.AddCommand(NewChartRepoAddCmd())

	return cmd
}

func NewChartRepoAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new chart repository",
		RunE: func(cc *cobra.Command, _ []string) error {
			var merr error
			if err := cc.MarkFlagRequired("name"); err != nil {
				merr = multierror.Append(merr, err)
			}
			if err := cc.MarkFlagRequired("url"); err != nil {
				merr = multierror.Append(merr, err)
			}

			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			name, err := flags.GetString("name")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			url, err := flags.GetString("url")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			usernameEnv, err := flags.GetString("username_env")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			passwordEnv, err := flags.GetString("password_env")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			caPath, err := flags.GetString("ca_path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			tlsClientCertDataPath, err := flags.GetString("tls_client_cert_data_path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			tlsClientCertKeyPath, err := flags.GetString("tls_client_cert_key_path")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			insecureSkipVerify, err := flags.GetBool("insecure_skip_verify")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			passCredentials, err := flags.GetBool("pass_credentials")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			logLevel, err := flags.GetString("log_level")
			if err != nil {
				merr = multierror.Append(merr, err)
			}
			quiet, err := flags.GetBool("quiet")
			if err != nil {
				merr = multierror.Append(merr, err)
			}

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)
			cr := &kclhelm.ChartRepo{
				Name:                  name,
				URL:                   url,
				UsernameEnv:           usernameEnv,
				PasswordEnv:           passwordEnv,
				CAPath:                caPath,
				TLSClientCertDataPath: tlsClientCertDataPath,
				TLSClientCertKeyPath:  tlsClientCertKeyPath,
				InsecureSkipVerify:    insecureSkipVerify,
				PassCredentials:       passCredentials,
			}

			if quiet || !isatty.IsTerminal(os.Stdout.Fd()) {
				return c.AddRepo(cr)
			}

			ct, err := helmtui.NewChartTUI(c, logLevel)
			if err != nil {
				return fmt.Errorf("failed to create tui: %w", err)
			}

			return ct.AddRepo(cr)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringP("name", "n", "", "Helm chart repository name (required)")
	cmd.Flags().StringP("url", "u", "", "URL of the Helm chart repository (required)")
	cmd.Flags().StringP("username_env", "U", "", "Basic authentication username environment variable")
	cmd.Flags().StringP("password_env", "P", "", "Basic authentication password environment variable")
	cmd.Flags().String("ca_path", "", "CA file path")
	cmd.Flags().String("tls_client_cert_data_path", "", "TLS client certificate data path")
	cmd.Flags().String("tls_client_cert_key_path", "", "TLS client certificate key path")
	cmd.Flags().Bool("insecure_skip_verify", false, "Skip SSL certificate verification")
	cmd.Flags().Bool("pass_credentials", false, "Pass credentials to the Helm chart repository")

	must(cmd.MarkFlagRequired("name"))
	must(cmd.MarkFlagRequired("url"))

	return cmd
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
