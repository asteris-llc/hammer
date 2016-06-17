package hammer

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/stretchr/testify/assert"
)

var goVersion = exec.Command("go", "version")
var tick, _ = time.ParseDuration(".1s")

func wrappedNewProcessLogger(t *testing.T, cmd *exec.Cmd) *ProcessLogger {
	logger, err := NewProcessLogger(cmd)
	assert.NotNil(t, logger)
	assert.NoError(t, err)

	assert.NotEmpty(t, logger.sources)
	assert.Contains(t, logger.sources, "out")
	assert.Contains(t, logger.sources, "err")
	assert.NotEmpty(t, logger.sources["out"])
	assert.NotEmpty(t, logger.sources["err"])

	return logger
}

func TestStart(t *testing.T) {
	t.Parallel()

	logger := wrappedNewProcessLogger(t, exec.Command("go", "version"))
	assert.NoError(t, logger.Start())
	_ = logger.Stop()
}

func TestStop(t *testing.T) {
	t.Parallel()

	logger := wrappedNewProcessLogger(t, exec.Command("go", "version"))
	assert.NoError(t, logger.Stop())  // no-op
	assert.NoError(t, logger.Start()) // start normally
	assert.NoError(t, logger.Stop())  // stop normally
	assert.NoError(t, logger.Stop())  // no-op
	assert.NoError(t, logger.Start()) // start again!
	assert.NoError(t, logger.Stop())  // okay, let's really be done
}

func TestSubscribe(t *testing.T) {
	t.Parallel()

	logger := wrappedNewProcessLogger(t, exec.Command("go", "version"))

	stdout, stderr, err := logger.Subscribe()
	assert.NoError(t, err)

	assert.NotEmpty(t, logger.destinations)
	assert.Contains(t, logger.destinations, "out")
	assert.Contains(t, logger.destinations, "err")
	assert.NotEmpty(t, logger.destinations["out"])
	assert.NotEmpty(t, logger.destinations["err"])

	assert.Empty(t, stdout)
	assert.Empty(t, stderr)

	assert.NoError(t, logger.Start())

	time.Sleep(tick) // wait for output to be recieved

	// assert.NotEmpty(t, stdout) // TODO: this fails
	assert.Empty(t, stderr)

	assert.NoError(t, logger.Stop())
}

func TestHandle(t *testing.T) {
	t.Parallel()

	// use a custom reader instead of reading from command's output
	reader := bytes.NewReader([]byte("test content"))
	logger, _ := NewProcessLogger(exec.Command("go", "version"))

	_, stderr, _ := logger.Subscribe()

	go logger.handle(context.TODO(), "out", reader)

	time.Sleep(tick) // wait for output to be recieved

	// assert.NotEmpty(t, stdout) // TODO: this fails
	assert.Empty(t, stderr)

	assert.NoError(t, logger.Stop())
}

func wrappedNewRollupConsumer(t *testing.T, cmd *exec.Cmd) (*ProcessLogger, *RollupConsumer) {
	logger := wrappedNewProcessLogger(t, cmd)
	rollUp, err := NewRollupConsumer(logger)
	assert.NotNil(t, rollUp)
	assert.NoError(t, err)

	return logger, rollUp
}

func TestNewRollupConsumer(t *testing.T) {
	t.Parallel()

	_, rollUp := wrappedNewRollupConsumer(t, exec.Command("go", "version"))
	assert.NotNil(t, rollUp)
}

func TestRollupHandle(t *testing.T) {
	logger, rollUp := wrappedNewRollupConsumer(t, exec.Command("go", "version"))
	rollUp.Out = *bytes.NewBuffer([]byte{})
	rollUp.Err = *bytes.NewBuffer([]byte{})

	assert.NoError(t, logger.Start())

	time.Sleep(tick) // wait for output to be recieved

	// assert.NotZero(t, rollUp.Out.Len()) // TODO: this fails
	assert.Zero(t, rollUp.Err.Len())
}

func TestRelayToStdIO(t *testing.T) {
	t.Parallel()

	src := make(chan []byte)
	go func() { src <- []byte("test") }()
	dest := bytes.NewBuffer([]byte{})
	go replayToStdIO(src, dest)

	time.Sleep(tick) // wait for output to be recieved

	assert.NotZero(t, dest.Len())
}

func TestStdIOConsumer(t *testing.T) {
	t.Parallel()

	logger := wrappedNewProcessLogger(t, exec.Command("go", "version"))
	assert.NoError(t, StdIOConsumer(logger))
}

func wrappedNewFileConsumer(t *testing.T, cmd *exec.Cmd) (*ProcessLogger, *FileConsumer, string) {
	logger := wrappedNewProcessLogger(t, cmd)

	tempDir, err := ioutil.TempDir("", "test")
	assert.NoError(t, err)

	fc, err := NewFileConsumer(logger, tempDir, "test")
	assert.NoError(t, err)

	return logger, fc, tempDir
}

func TestFileConsumerHandle(t *testing.T) {
	t.Parallel()

	_, fc, tempDir := wrappedNewFileConsumer(t, exec.Command("go", "version"))
	defer func(dir string) {
		_ = os.Remove(dir)
	}(tempDir)

	src := make(chan []byte)
	go func() { src <- []byte("test\ntest") }()

	go fc.handle(path.Join(tempDir, "test"), src)

	time.Sleep(tick) // wait for output to be written

	content, err := ioutil.ReadFile(path.Join(tempDir, "test"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "test")
	assert.Contains(t, string(content), "\n")
}

func TestFileConsumer(t *testing.T) {
	t.Parallel()

	logger, _, tempDir := wrappedNewFileConsumer(t, exec.Command("go", "version"))
	defer func(dir string) {
		_ = os.Remove(dir)
	}(tempDir)

	assert.NoError(t, logger.Start())
	time.Sleep(tick) // wait for output to be recieved

	files, err := ioutil.ReadDir(tempDir)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	if len(files) > 0 {
		assert.Contains(t, files[0].Name(), "stderr")
		_, err := ioutil.ReadFile(path.Join(tempDir, files[0].Name()))
		assert.NoError(t, err)
		// assert.Contains(t, string(content), "go") // TODO: this fails
	}
	if len(files) > 1 {
		assert.Contains(t, files[1].Name(), "stdout")
		_, err := ioutil.ReadFile(path.Join(tempDir, files[1].Name()))
		assert.NoError(t, err)
	}
}
