package hammer

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Loader struct {
	Root      string
	Indicator string
}

func NewLoader(root string) *Loader {
	return &Loader{
		Root:      root,
		Indicator: "spec.yml",
	}
}

func (l *Loader) Load() ([]*Package, error) {
	logrus.WithField("root", l.Root).Info("loading packages")
	packages := []*Package{}

	err := filepath.Walk(l.Root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Name() != l.Indicator {
			return nil
		}

		logrus.WithField("path", path).Debug("loading package")
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		pkg, err := NewPackageFromYAML(content)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":  path,
				"error": err,
			}).Warning("could not load package, skipping")
			return nil
		}
		pkg.Root = path
		pkg.OutputRoot = viper.GetString("output")

		packages = append(packages, pkg)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return packages, nil
}
