package exec

import (
	"os/exec"
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestRunCommand(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	message, err := RunCommand("echo", CmdOpts{Redactor: Redact([]string{"world"})}, "hello world")
	assert.NoError(t, err)
	assert.Equal(t, "hello world", message)

	assert.Len(t, hook.Entries, 2)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "echo hello ******", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello ******\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")
}

func TestRunCommandTimeout(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Timeout: 500 * time.Millisecond}, "-c", "echo hello world && echo my-error >&2 && sleep 2")
	assert.Equal(t, output, "hello world")
	assert.EqualError(t, err, "`sh -c echo hello world && echo my-error >&2 && sleep 2` failed timeout after 500ms")

	assert.Len(t, hook.Entries, 3)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "sh -c echo hello world && echo my-error >&2 && sleep 2", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello world\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[2]
	assert.Equal(t, log.ErrorLevel, entry.Level)
	assert.Equal(t, "`sh -c echo hello world && echo my-error >&2 && sleep 2` failed timeout after 500ms", entry.Message)
	assert.Contains(t, entry.Data, "execID")
}

func TestRunCommandSignal(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	timeoutBehavior := TimeoutBehavior{Signal: syscall.SIGTERM, ShouldWait: true}
	output, err := RunCommand("sh", CmdOpts{Timeout: 200 * time.Millisecond, TimeoutBehavior: timeoutBehavior}, "-c", "trap 'trap - 15 && echo captured && exit' 15 && sleep 2")
	assert.Equal(t, "captured", output)
	assert.EqualError(t, err, "`sh -c trap 'trap - 15 && echo captured && exit' 15 && sleep 2` failed timeout after 200ms")

	assert.Len(t, hook.Entries, 3)
}

func TestTrimmedOutput(t *testing.T) {
	message, err := RunCommand("printf", CmdOpts{}, "hello world")
	assert.NoError(t, err)
	assert.Equal(t, "hello world", message)
}

func TestRunCommandExitErr(t *testing.T) {
	hook := test.NewGlobal()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Redactor: Redact([]string{"world"})}, "-c", "echo hello world && echo my-error >&2 && exit 1")
	assert.Equal(t, "hello world", output)
	assert.EqualError(t, err, "`sh -c echo hello ****** && echo my-error >&2 && exit 1` failed exit status 1: my-error")

	assert.Len(t, hook.Entries, 3)

	entry := hook.Entries[0]
	assert.Equal(t, log.InfoLevel, entry.Level)
	assert.Equal(t, "sh -c echo hello ****** && echo my-error >&2 && exit 1", entry.Message)
	assert.Contains(t, entry.Data, "dir")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[1]
	assert.Equal(t, log.DebugLevel, entry.Level)
	assert.Equal(t, "hello ******\n", entry.Message)
	assert.Contains(t, entry.Data, "duration")
	assert.Contains(t, entry.Data, "execID")

	entry = hook.Entries[2]
	assert.Equal(t, log.ErrorLevel, entry.Level)
	assert.Equal(t, "`sh -c echo hello ****** && echo my-error >&2 && exit 1` failed exit status 1: my-error", entry.Message)
	assert.Contains(t, entry.Data, "execID")
}

func TestRunCommandErr(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(log.InfoLevel)

	output, err := RunCommand("sh", CmdOpts{Redactor: Redact([]string{"world"})}, "-c", ">&2 echo 'failure'; false")
	assert.Empty(t, output)
	assert.EqualError(t, err, "`sh -c >&2 echo 'failure'; false` failed exit status 1: failure")
}

func TestRunInDir(t *testing.T) {
	cmd := exec.Command("pwd")
	cmd.Dir = "/"
	message, err := RunCommandExt(cmd, CmdOpts{})
	assert.Nil(t, err)
	assert.Equal(t, "/", message)
}

func TestRedact(t *testing.T) {
	assert.Equal(t, "", Redact(nil)(""))
	assert.Equal(t, "", Redact([]string{})(""))
	assert.Equal(t, "", Redact([]string{"foo"})(""))
	assert.Equal(t, "foo", Redact([]string{})("foo"))
	assert.Equal(t, "******", Redact([]string{"foo"})("foo"))
	assert.Equal(t, "****** ******", Redact([]string{"foo", "bar"})("foo bar"))
	assert.Equal(t, "****** ******", Redact([]string{"foo"})("foo foo"))
}

func TestRunCaptureStderr(t *testing.T) {
	output, err := RunCommand("sh", CmdOpts{CaptureStderr: true}, "-c", "echo hello world && echo my-error >&2 && exit 0")
	assert.Equal(t, "hello world\nmy-error", output)
	assert.NoError(t, err)
}
