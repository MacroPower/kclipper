package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/MacroPower/kclipper/pkg/kclexport"
)

const (
	exportDesc = `This command converts KCL schemas to other formats.
`
	exportExample = `  kcl export <command> [arguments]...
  # Export a schema in the current package
  kcl export -m jsonschema -S path.to.MySchema

  # Export a schema in another package
  kcl export path/to -m jsonschema -S MySchema
`
)

// NewExportCmd returns the export command.
func NewExportCmd(arg *RootArgs) *cobra.Command {
	args := NewExportArgs(arg)

	cmd := &cobra.Command{
		Use:          "export",
		Short:        "KCL export tool",
		Long:         exportDesc,
		Example:      exportExample,
		SilenceUsage: true,
		Args:         cobra.MatchAll(cobra.RangeArgs(0, 1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, pArgs []string) error {
			err := cmd.ValidateArgs(pArgs)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}

			pkgPath := "."
			if len(pArgs) > 0 {
				pkgPath = pArgs[0]
			}

			js, err := kclexport.Export.KCLSchemaToJSONSchema(pkgPath, args.GetSchema())
			if err != nil {
				return fmt.Errorf("failed to export schema: %w", err)
			}

			outFile := args.GetOutput()

			// If no output file is specified, print to stdout.
			if outFile == "" {
				cmd.Println(string(js))

				return nil
			}

			err = os.MkdirAll(filepath.Dir(outFile), 0o700)
			if err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			err = os.WriteFile(outFile, js, 0o600)
			if err != nil {
				return fmt.Errorf("failed to write to output file: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(args.mode, "mode", "m", "jsonschema", "Specify the export mode")

	cmd.Flags().StringVarP(args.schema, "schema", "S", "", "Specify the root schema selector")
	must(cmd.MarkFlagRequired("schema"))

	cmd.Flags().StringVarP(args.output, "output", "o", "", "Specify the output file path")
	must(cmd.MarkFlagFilename("output"))

	return cmd
}

// ExportArgs holds the arguments for the export command.
type ExportArgs struct {
	output *string
	mode   *string
	schema *string
	*RootArgs
}

// NewExportArgs creates a new [ExportArgs].
func NewExportArgs(args *RootArgs) *ExportArgs {
	return &ExportArgs{
		output:   new(string),
		mode:     new(string),
		schema:   new(string),
		RootArgs: args,
	}
}

func (a *ExportArgs) GetOutput() string {
	return *a.output
}

func (a *ExportArgs) GetMode() string {
	return *a.mode
}

func (a *ExportArgs) GetSchema() string {
	return *a.schema
}
