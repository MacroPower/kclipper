package os

import (
	"bytes"
	"fmt"
	"os/exec"
)

type ExecOutput struct {
	Stdout string
	Stderr string
}

func Exec(name string, arg []string, env []string) (*ExecOutput, error) {
	cmd := exec.Command(name, arg...)
	cmd.Env = env

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute %s: %w", name, err)
	}

	return &ExecOutput{
		Stdout: outb.String(),
		Stderr: errb.String(),
	}, nil
}
