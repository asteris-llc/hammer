package hammer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"time"

	"golang.org/x/net/context"
)

// ProcessLogger logs a given process
type ProcessLogger struct {
	sources      map[string]io.Reader
	destinations map[string][]chan []byte

	errors chan error
	cancel func()
}

// NewProcessLogger creates a ProcessLogger when given a process
func NewProcessLogger(cmd *exec.Cmd) (*ProcessLogger, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	logger := &ProcessLogger{
		sources: map[string]io.Reader{
			"out": stdout,
			"err": stderr,
		},
		destinations: map[string][]chan []byte{
			"out": []chan []byte{make(chan []byte)},
			"err": []chan []byte{make(chan []byte)},
		},

		errors: make(chan error, 2),
	}

	return logger, nil
}

// Subscribe returns two channels for the logged command's stdout and stderr
func (p *ProcessLogger) Subscribe() (stdout chan []byte, stderr chan []byte, err error) {
	if p.cancel != nil {
		return nil, nil, errors.New("already started")
	} else if len(p.destinations["out"]) < 1 || len(p.destinations["err"]) < 1 {
		return nil, nil, errors.New("not properly initialized")
	}
	return p.destinations["out"][0], p.destinations["err"][0], nil
}

// Start the process after subscribers are registered
func (p *ProcessLogger) Start() error {
	var ctx context.Context
	ctx, p.cancel = context.WithCancel(context.Background())

	for name, source := range p.sources {
		go p.handle(ctx, name, source)
	}

	select {
	case err := <-p.errors:
		return err
	default:
		return nil
	}
}

// Stop the process after the command is finished to clean up goroutines
func (p *ProcessLogger) Stop() error {
	if p == nil || p.cancel == nil {
		return nil
	}

	p.cancel()

	select {
	case err := <-p.errors:
		return err
	default:
		return nil
	}
}

func (p *ProcessLogger) handle(ctx context.Context, name string, source io.Reader) {
	defer func() {
		for _, dest := range p.destinations[name] {
			close(dest)
		}
	}()

	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for _, dest := range p.destinations[name] {
			select {
			case <-ctx.Done():
				return
			case dest <- scanner.Bytes():
			}
		}
	}

	if err := scanner.Err(); err != nil {
		select {
		case <-ctx.Done():
			return
		case p.errors <- err:
		}
	}
}

// RollupConsumer keeps a tally of the logs written so far in memory
type RollupConsumer struct {
	Out bytes.Buffer
	Err bytes.Buffer

	errors chan error
}

// NewRollupConsumer gets a new RollupConsumer
func NewRollupConsumer(p *ProcessLogger) (*RollupConsumer, error) {
	stdout, stderr, err := p.Subscribe()
	if err != nil {
		return nil, err
	}

	r := &RollupConsumer{errors: make(chan error)}
	go r.handle(stdout, r.Out)
	go r.handle(stderr, r.Err)

	return r, nil
}

func (r *RollupConsumer) handle(src chan []byte, dest bytes.Buffer) {
	for line := range src {
		if _, err := dest.Write(line); err != nil {
			r.errors <- err
		}
	}
}

// StdIOConsumer replays logs written to stdio
func StdIOConsumer(p *ProcessLogger) error {
	stdout, stderr, err := p.Subscribe()
	if err != nil {
		return err
	}

	go replayToStdIO(stdout, os.Stdout)
	go replayToStdIO(stderr, os.Stderr)

	return nil
}

func replayToStdIO(src chan []byte, dest io.Writer) {
	for line := range src {
		fmt.Fprintln(dest, string(line))
	}
}

// FileConsumer logs files to the given directory
type FileConsumer struct {
	errs chan error
}

// NewFileConsumer starts a FileConsumer with the given options
func NewFileConsumer(p *ProcessLogger, dest, name string) (*FileConsumer, error) {
	stdout, stderr, err := p.Subscribe()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dest, os.ModeDir|0777); err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)

	f := &FileConsumer{make(chan error)}

	go f.handle(path.Join(dest, fmt.Sprintf("%s-%s-stdout.log", name, now)), stdout)
	go f.handle(path.Join(dest, fmt.Sprintf("%s-%s-stderr.log", name, now)), stderr)

	if err := f.Error(); err != nil {
		return f, err
	}

	return f, nil
}

func (f *FileConsumer) Error() error {
	select {
	case err := <-f.errs:
		return err
	default:
		return nil
	}
}

func (f *FileConsumer) handle(filename string, src chan []byte) {
	file, err := os.Create(filename)
	if err != nil {
		f.errs <- err
		return
	}

	// always close the file and handle the possible error
	defer func(f *FileConsumer, file *os.File) {
		if err := file.Close(); err != nil {
			f.errs <- err
		}
	}(f, file)

	for line := range src {
		_, err = file.Write(append(line, []byte("\n")...))
		if err != nil {
			f.errs <- err
			return
		}
	}
}
