package os

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

type ExecOutput struct {
	Stdout string
	Stderr string
}

func Exec(name string, arg ...string) (ExecOutput, error) {
	cmd := exec.Command(name, arg...)

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := errors.Wrapf(cmd.Run(), "failed to execute %s", name)

	return ExecOutput{
		Stdout: outb.String(),
		Stderr: errb.String(),
	}, err
}
