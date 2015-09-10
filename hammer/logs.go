package hammer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"time"
)

type LogConsumer interface {
	MustHandleStream(string, io.Reader)
	HandleStream(string, io.Reader) error
}

type FileConsumer struct {
	Name   string
	Output string
}

func NewFileConsumer(name string, path string) (LogConsumer, error) {
	err := os.MkdirAll(path, os.ModeDir|0777)
	if err != nil {
		return nil, err
	}

	return &FileConsumer{name, path}, nil
}

func (fc *FileConsumer) MustHandleStream(description string, stream io.Reader) {
	err := fc.HandleStream(description, stream)
	if err != nil {
		panic(err)
	}
}

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
