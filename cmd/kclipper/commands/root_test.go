package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/macropower/kclipper/cmd/kclipper/commands"
)

var testDataDir string

func init() {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	testDataDir = filepath.Join(dir, "testdata")
}

func TestRunCmd(t *testing.T) {
	err := os.RemoveAll(filepath.Join(testDataDir, "got/run_cmd"))
	require.NoError(t, err)

	tc := commands.NewRootCmd("test_run", "", "")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	outFile := filepath.Join(testDataDir, "got/run_cmd/simple.json")
	err = os.MkdirAll(filepath.Dir(outFile), 0o750)
	require.NoError(t, err)

	tc.SetArgs([]string{
		"run", filepath.Join(testDataDir, "simple.k"),
		"--format=json",
		"--output", outFile,
	})
	tc.SetOut(stdout)
	tc.SetErr(stderr)

	err = tc.Execute()
	require.NoError(t, err)
	assert.Empty(t, stderr.String(), "stderr should be empty")
	assert.Empty(t, stdout.String(), "stdout should be empty")

	outData, err := os.ReadFile(outFile)
	require.NoError(t, err)

	require.JSONEq(t, `{"a":1}`, string(outData))
}

func BenchmarkRun(b *testing.B) {
	for b.Loop() {
		tc := commands.NewRootCmd("bench_run", "", "")
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		tc.SetArgs([]string{"run", filepath.Join(testDataDir, "simple.k"), "--output=/dev/null"})
		tc.SetOut(stdout)
		tc.SetErr(stderr)

		err := tc.Execute()
		require.NoError(b, err)
		assert.Empty(b, stderr.String(), "stderr should be empty")
		assert.Empty(b, stdout.String(), "stdout should be empty")
	}
}

func TestRootCmdArgs(t *testing.T) {
	tcs := map[string]struct {
		wantErr   error
		logLevel  string
		logFormat string
	}{
		"default config": {
			logLevel:  "warn",
			logFormat: "text",
		},
		"json format": {
			logLevel:  "info",
			logFormat: "json",
		},
		"debug level": {
			logLevel:  "debug",
			logFormat: "text",
		},
		"invalid log level": {
			logLevel:  "invalid",
			logFormat: "text",
			wantErr:   commands.ErrLogHandlerFailed,
		},
		"invalid log format": {
			logLevel:  "warn",
			logFormat: "invalid",
			wantErr:   commands.ErrLogHandlerFailed,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			rootCmd := commands.NewRootCmd("test_logger", "", "")
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			rootCmd.SetArgs([]string{
				"--log_level", tc.logLevel,
				"--log_format", tc.logFormat,
				"version",
			})
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)

			err := rootCmd.Execute()

			if tc.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
				assert.Regexp(t, `\d+\.\d+\.\d+`, stdout.String())
			}
		})
	}
}

func TestRootCmdArgPointers(t *testing.T) {
	args := commands.NewRootArgs()

	// Test default values
	assert.Empty(t, args.GetLogLevel())
	assert.Empty(t, args.GetLogFormat())
}
