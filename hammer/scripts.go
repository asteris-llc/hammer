package hammer

import (
	"bytes"
	"errors"
	"github.com/Sirupsen/logrus"
)

var (
	ErrNoScript = errors.New("no such script")
)

type Scripts map[string]string

func (s Scripts) Content(p *Package, name string) (bytes.Buffer, error) {
	source, ok := s[name]
	if !ok {
		return bytes.Buffer{}, ErrNoScript
	}

	out, err := p.template.Render(source)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"name":  name,
			"error": err,
		}).Error("could not template script")
		return bytes.Buffer{}, err
	}

	return out, nil
}
