package hammer

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"time"
)

// LogConsumer is an interface that should be able to take build logs and put
// them somewhere else in a streaming fashion.
type LogConsumer interface {
	MustHandleStream(string, io.Reader)
	HandleStream(string, io.Reader) error
	Replay(string) ([]byte, error)
}

// FileConsumer streams logs to disk, line by line.
type FileConsumer struct {
	ID     string
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

	return &FileConsumer{time.Now().Format(time.RFC3339), name, path}, nil
}

// Replay gives a current replay of the logs from a description
func (fc *FileConsumer) Replay(description string) ([]byte, error) {
	return ioutil.ReadFile(fc.Location(description))
}

// Location gives the destination location of a named string
func (fc *FileConsumer) Location(description string) string {
	return path.Join(
		fc.Output,
		fmt.Sprintf("%s-%s-%s.log", fc.Name, description, fc.ID),
	)
}

// HandleStream takes a description of a stream and the stream itself and writes
// it line-by-line to disk.
func (fc *FileConsumer) HandleStream(description string, stream io.Reader) error {
	file, err := os.Create(fc.Location(description))
	if err != nil {
		return err
	}

	newline := byte('\n')
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		_, err := file.Write(append(scanner.Bytes(), newline))
		if err != nil {
			return err
		}
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
