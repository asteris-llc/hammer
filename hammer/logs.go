package hammer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"time"
)

// LogConsumer is an interface that should be able to take build logs and put
// them somewhere else in a streaming fashion.
type LogConsumer interface {
	MustHandleStream(string, io.Reader)
	HandleStream(string, io.Reader) error
}

// FileConsumer streams logs to disk, line by line.
type FileConsumer struct {
	Name   string
	Output string
}

// NewFileConsumer creates the output directory for logs and returns the
// FileConsumer, ready for action.
func NewFileConsumer(name string, path string) (LogConsumer, error) {
	err := os.MkdirAll(path, os.ModeDir|0777)
	if err != nil {
		return nil, err
	}

	return &FileConsumer{name, path}, nil
}

// HandleStream takes a description of a stream and the stream itself and writes
// it line-by-line to disk.
func (fc *FileConsumer) HandleStream(description string, stream io.Reader) error {
	now := time.Now().Format(time.RFC3339)

	file, err := os.Create(path.Join(
		fc.Output,
		fmt.Sprintf("%s-%s-%s.log", fc.Name, description, now),
	))
	if err != nil {
		return err
	}

	newline := []byte("\n")
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		file.Write(scanner.Bytes())
		file.Write(newline)
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// MustHandleStream does the same thing as HandleStream but will panic for any errors.
func (fc *FileConsumer) MustHandleStream(description string, stream io.Reader) {
	err := fc.HandleStream(description, stream)
	if err != nil {
		panic(err)
	}
}
