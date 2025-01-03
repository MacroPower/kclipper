package cli

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/MacroPower/kclipper/pkg/helm"
	"github.com/MacroPower/kclipper/pkg/helmutil"
	"github.com/MacroPower/kclipper/pkg/jsonschema"
)

const (
	chartDesc = `This command manages kcl charts
`
	chartExample = `  kcl chart <command> [arguments]...
  # Initialize the current module
  kcl chart init

  # Add chart for the current module
  kcl chart add --chart podinfo --repo_url https://stefanprodan.github.io/podinfo --target_revision 6.7.0

  # Update chart schemas for the current module
  kcl chart update`
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
	_ = cmd.MarkFlagDirname("path")
	cmd.AddCommand(NewChartInitCmd())
	cmd.AddCommand(NewChartAddCmd())
	cmd.AddCommand(NewChartUpdateCmd())

	return cmd
}

func NewChartInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the current module",
		RunE: func(cc *cobra.Command, _ []string) error {
			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}
			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)
			return c.Init()
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

			if merr != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, merr)
			}

			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)
			return c.Add(chart, repoURL, targetRevision, schemaPath, schemaGenerator, schemaValidator)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringP("chart", "c", "", "Helm chart name (required)")
	cmd.Flags().StringP("repo_url", "r", "", "URL of the Helm chart repository (required)")
	cmd.Flags().StringP("target_revision", "t", "", "Semver tag for the chart's version")
	cmd.Flags().StringP("schema_generator", "G", "AUTO", "Chart schema generator")
	cmd.Flags().StringP("schema_validator", "V", "KCL", "Chart schema validator")
	cmd.Flags().StringP("schema_path", "P", "", "Chart schema path")

	return cmd
}

func NewChartUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update charts",
		RunE: func(cc *cobra.Command, _ []string) error {
			flags := cc.Flags()
			basePath, err := flags.GetString("path")
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}
			c := helmutil.NewChartPkg(basePath, helm.DefaultClient)
			return c.Update()
		},
		SilenceUsage: true,
	}
}
