package hammer

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"golang.org/x/net/context"
)

// ProcessLogger logs a given process
type ProcessLogger struct {
	sources      map[string]io.ReadCloser
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
		sources:      map[string]io.ReadCloser{"out": stdout, "err": stderr},
		destinations: map[string][]chan []byte{},

		errors: make(chan error, 2),
	}

	return logger, nil
}

// Subscribe returns two channels for the logged command's stdout and stderr
func (p *ProcessLogger) Subscribe() (stdout chan []byte, stderr chan []byte, err error) {
	if p.cancel != nil {
		return nil, nil, errors.New("already started")
	}
	stdout = make(chan []byte)
	stderr = make(chan []byte)

	p.destinations["out"] = append(p.destinations["out"], stdout)
	p.destinations["err"] = append(p.destinations["err"], stderr)

	return
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
	p.cancel()

	select {
	case err := <-p.errors:
		return err
	default:
		return nil
	}
}

func (p *ProcessLogger) handle(ctx context.Context, name string, source io.ReadCloser) {
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
}

// NewRollupConsumer gets a new RollupConsumer
func NewRollupConsumer(p *ProcessLogger) (*RollupConsumer, error) {
	stdout, stderr, err := p.Subscribe()
	if err != nil {
		return nil, err
	}

	r := new(RollupConsumer)
	go r.handle(stdout, r.Out)
	go r.handle(stderr, r.Err)

	return r, nil
}

func (r *RollupConsumer) handle(src chan []byte, dest bytes.Buffer) {
	for line := range src {
		dest.Write(line)
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
func NewFileConsumer(p *ProcessLogger, path, name string) (*FileConsumer, error) {
	stdout, stderr, err := p.Subscribe()
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(path, os.ModeDir|0777)
	if err != nil {
		return nil, err
	}

	time := time.Now().Format(time.RFC3339)

	f := &FileConsumer{make(chan error)}

	go f.handle(fmt.Sprintf("%s/%s-%s-stdout.log", path, name, time), stdout)
	go f.handle(fmt.Sprintf("%s/%s-%s-stderr.log", path, name, time), stderr)

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
	defer file.Close()

	for line := range src {
		_, err = file.Write(line)
		if err != nil {
			f.errs <- err
			return
		}
	}
}
