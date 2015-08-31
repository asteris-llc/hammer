package hammer

import (
	"bytes"
	"errors"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"path"
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

func (s Scripts) RenderAndWriteAll(p *Package) (map[string]string, error) {
	locations := map[string]string{}

	for name, _ := range s {
		content, err := s.Content(p, name)
		if err != nil {
			return locations, err
		}

		dest := path.Join(p.ScriptRoot, name)
		err = ioutil.WriteFile(dest, content.Bytes(), 0777)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"name":  name,
				"error": err,
			}).Error("could not write script to disk")
			return locations, err
		}

		locations[name] = dest
	}

	return locations, nil
}
