package os

import (
	"bytes"
	"fmt"
	"os"
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
		return nil, fmt.Errorf("run error: %w", err)
	}

	return &ExecOutput{
		Stdout: outb.String(),
		Stderr: errb.String(),
	}, nil
}

func GetEnv(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("env var '%s' not found", key)
	}
	return v, nil
}
