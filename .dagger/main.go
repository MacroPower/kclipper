// Development environment for kclipper.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// KclipperDev is the root development module for kclipper.
type KclipperDev struct{}

// Generated verifies that generated code is up to date.
//
// +check
func (dev *KclipperDev) Generated(ctx context.Context) error {
	generated := dag.CurrentModule().Generators().Run()
	empty, err := generated.IsEmpty(ctx)
	if err != nil {
		return err
	}
	if !empty {
		changes := generated.Changes()
		patch, err := changes.AsPatch().Contents(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, patch)
		return errors.New("generated files are not up-to-date")
	}
	return nil
}
