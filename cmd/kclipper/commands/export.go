package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/macropower/kclipper/pkg/kclexport"
)

var (
	// ErrExportSchema indicates an error occurred while exporting a schema.
	ErrExportSchema = errors.New("export schema")

	// ErrExportOutput indicates an error occurred while writing export output.
	ErrExportOutput = errors.New("export output")
)

const (
	exportDesc = `This command converts KCL schemas to other formats.
`
	exportExample = `  kcl export <command> [arguments]...
  # Export a schema in the current package
  kcl export -S path.to.MySchema

  # Export a schema in another package
  kcl export path/to -S MySchema
`
)

// NewExportCmd returns the export command.
func NewExportCmd() *cobra.Command {
	args := NewExportArgs()

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "KCL export tool",
		Long:    exportDesc,
		Example: exportExample,
		Args:    cobra.MatchAll(cobra.RangeArgs(0, 1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, pArgs []string) error {
			err := cmd.ValidateArgs(pArgs)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrInvalidArgument, err)
			}

			pkgPath := "."
			if len(pArgs) > 0 {
				pkgPath = pArgs[0]
			}

			js, err := kclexport.KCLSchemaToJSONSchema(pkgPath, args.GetSchema())
			if err != nil {
				return fmt.Errorf("%w: %w", ErrExportSchema, err)
			}

			outFile := args.GetOutput()

			// If no output file is specified, print to stdout.
			if outFile == "" {
				cmd.Println(string(js))

				return nil
			}

			err = os.MkdirAll(filepath.Dir(outFile), 0o700)
			if err != nil {
				return fmt.Errorf("%w: create directory: %w", ErrExportOutput, err)
			}

			err = os.WriteFile(outFile, js, 0o600)
			if err != nil {
				return fmt.Errorf("%w: write file: %w", ErrExportOutput, err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(args.schema, "schema", "S", "", "Specify the root schema selector")
	must(cmd.MarkFlagRequired("schema"))

	cmd.Flags().StringVarP(args.output, "output", "o", "", "Specify the output file path")
	must(cmd.MarkFlagFilename("output"))

	return cmd
}

// ExportArgs holds the arguments for the export command.
// Create instances with [NewExportArgs].
type ExportArgs struct {
	output *string
	schema *string
}

// NewExportArgs creates a new [ExportArgs].
func NewExportArgs() *ExportArgs {
	return &ExportArgs{
		output: new(string),
		schema: new(string),
	}
}

func (a *ExportArgs) GetOutput() string {
	return *a.output
}

func (a *ExportArgs) GetSchema() string {
	return *a.schema
}
