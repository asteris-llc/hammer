package hammer

import (
	"bytes"
	"errors"
	"github.com/Sirupsen/logrus"
	"io/ioutil"
	"path"
)

var (
	// ErrNoScript is returend when the requested script isn't found.
	ErrNoScript = errors.New("no such script")
)

// Scripts is just an alias for a map[string]string - this means that you can
// specify any script names you like (whether they'll be accepted, however, is a
// different story)
type Scripts map[string]string

// Content reads the given script name off disk and returns it's rendered
// content
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

// RenderAndWriteAll renders all scripts and writes them to the ScriptRoot of
// the given Package. It returns a map of the script names to their locations on
// disk.
func (s Scripts) RenderAndWriteAll(p *Package) (map[string]string, error) {
	locations := map[string]string{}

	for name := range s {
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
