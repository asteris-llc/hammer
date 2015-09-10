package hammer

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Loader looks for Package specs in a given root.
type Loader struct {
	// The loader looks for packages in Root
	Root string

	// The loader looks for files named the value of Indicator to signify a package
	Indicator string
}

// NewLoader returns a Loader with default values set
func NewLoader(root string) *Loader {
	return &Loader{
		Root:      root,
		Indicator: "spec.yml",
	}
}

// Load finds all the packages below Root in the filesystem
func (l *Loader) Load() ([]*Package, error) {
	logrus.WithField("root", l.Root).Info("loading packages")
	packages := []*Package{}

	err := filepath.Walk(l.Root, func(pathName string, info os.FileInfo, err error) error {
		if info.IsDir() || info.Name() != l.Indicator {
			return nil
		}

		logrus.WithField("path", pathName).Debug("loading package")
		content, err := ioutil.ReadFile(pathName)
		if err != nil {
			return err
		}

		pkg, err := NewPackageFromYAML(content)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"path":  pathName,
				"error": err,
			}).Warning("could not load package, skipping")
			return nil
		}
		path, _ := filepath.Split(pathName)
		pkg.SpecRoot = path
		pkg.OutputRoot = viper.GetString("output")
		pkg.LogRoot = viper.GetString("logs")

		packages = append(packages, pkg)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return packages, nil
}
