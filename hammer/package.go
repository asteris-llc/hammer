package hammer

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/asteris-llc/hammer/hammer/cache"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

// Target describes the output of a build. It has a source (Src) and a
// destination (Dest), and can be templated and marked as a config file.
type Target struct {
	Src      string `yaml:"src"`
	Dest     string `yaml:"dest"`
	Template bool   `yaml:"template"`
	Config   bool   `yaml:"config"`
}

// Package is the main struct in Hammer. It contains all the (meta-)information
// needed to produce a package.
type Package struct {
	Name         string     `yaml:"name"`
	Version      string     `yaml:"version"`
	Iteration    string     `yaml:"iteration"`
	Epoch        string     `yaml:"epoch"`
	License      string     `yaml:"license"`
	Vendor       string     `yaml:"vendor"`
	URL          string     `yaml:"url"`
	Description  string     `yaml:"description"`
	Architecture string     `yaml:"architecture"`
	Depends      []string   `yaml:"depends"`
	Resources    []Resource `yaml:"resources"`
	Targets      []Target   `yaml:"targets"`
	Scripts      Scripts    `yaml:"scripts"`
	ExtraArgs    string     `yaml:"extra-args"`

	// various roots
	BuildRoot  string `yaml:"-"`
	Empty      string `yaml:"-"`
	OutputRoot string `yaml:"-"`
	ScriptRoot string `yaml:"-"`
	SpecRoot   string `yaml:"-"`
	TargetRoot string `yaml:"-"`
	LogRoot    string `yaml:"-"`

	// information about the machine doing the building
	CPUs int `yaml:"-"`

	cache           cache.Cache
	logger          *logrus.Entry
	logconsumer     LogConsumer
	template        *Template
	scriptLocations map[string]string
	fpm             *FPM
}

// NewPackageFromYAML loads a package from YAML if it can. It also sets up
// defaults for build machine information, the logger, and the templater
func NewPackageFromYAML(content []byte) (*Package, error) {
	p := new(Package)
	err := yaml.Unmarshal(content, p)

	if err != nil {
		return p, err
	}

	// machine information
	p.CPUs = runtime.NumCPU()

	// private fields
	p.SetLogger(logrus.StandardLogger())
	p.SetTemplate(NewTemplate(p))

	return p, nil
}

// setters

// SetLogger sets the package logger from a plain logrus Logger, and sets a
// "name" field before accepting it.
func (p *Package) SetLogger(logger *logrus.Logger) {
	p.logger = logger.WithField("name", p.Name)
}

// SetTemplate sets the default template renderer for the package
func (p *Package) SetTemplate(tmpl *Template) {
	p.template = tmpl
}

// SetCache sets the cache for the package
func (p *Package) SetCache(cache cache.Cache) {
	p.cache = cache
}

// process

// BuildAndPackage is the main function you'll want to call after loading a
// Package. It takes care of all the stages of the build, including setup and
// cleanup.
func (p *Package) BuildAndPackage() error {
	defer func() {
		err := p.Cleanup()
		if err != nil {
			p.logger.Warn("did not clean up successfully, there may be residual files in your system")
		}
	}()
	type Stage struct {
		Name   string
		Action func() error
	}
	stages := []Stage{
		{"setup", p.Setup},
		{"build", p.Build},
		{"package", p.Package},
	}

	for _, stage := range stages {
		logger := p.logger.WithField("stage", stage.Name)
		logger.Debugf("starting %s stage", stage.Name)
		err := stage.Action()
		if err != nil {
			logger.WithField("error", err).Error("could not complete stage")
			return err
		}
		logger.Infof("finished %s stage", stage.Name)
	}

	return nil
}

// Setup does all the filesystem work necessary to make sure that the package
// can be built. This includes:
//
// - creating all temporary directories
// - getting the sources and storing them
// - rendering and writing all the scripts to disk
// - setting up the build logging
// - making sure FPM has an environment it can run in
func (p *Package) Setup() error {
	roots := map[string]*string{
		"build":  &p.BuildRoot,
		"script": &p.ScriptRoot,
		"target": &p.TargetRoot,
		"empty":  &p.Empty,
	}

	for name, root := range roots {
		dir, err := ioutil.TempDir("", fmt.Sprintf("hammer-%s-%s", p.Name, name))
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"root":  name,
			}).Errorf("could not create temporary directory")
			return err
		}

		*root = dir
	}

	// get the sources and store them in the temporary directory
	err := p.downloadResources()
	if err != nil {
		return err
	}

	// render scripts to disk
	locations, err := p.Scripts.RenderAndWriteAll(p)
	if err != nil {
		return err
	}
	p.scriptLocations = locations

	// create an FPM instance
	fpm, err := NewFPM(p)
	if err != nil {
		return err
	}
	p.fpm = fpm

	// create a log consumer
	consumer, err := NewFileConsumer(p.Name, p.LogRoot)
	if err != nil {
		return err
	}
	p.logconsumer = consumer

	return nil
}

func (p *Package) downloadResources() error {
	for _, s := range p.Resources {
		url := url.QueryEscape(s.RenderURL(p))
		name := s.Name(p)
		// TODO: check the hash from the content returned from disk, as well. Minor bug.
		p.logger.WithField("name", name).Debug("checking for resource")
		content, err := p.cache.Get(url)
		if err == cache.ErrNoSuchKey {
			p.logger.WithField("name", name).Debug("resource not found, downloading")
			content, err = s.Download(p)
			if err != nil {
				return err
			}

			err = p.cache.Set(url, content)
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"error": err,
					"name":  name,
				}).Error("could not cache response")
				return err
			}
		} else if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"name":  name,
			}).Error("could not get resource from cache")
			return err
		}

		err = ioutil.WriteFile(
			path.Join(p.BuildRoot, s.Name(p)),
			content,
			0777,
		)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"name":  name,
			}).Error("could not write resource to disk")
		}
	}

	return nil
}

// Cleanup is basically the opposite function of Setup, although it doesn't have
// nearly as much work to do. It just recursively removes the temporary
// directories.
func (p *Package) Cleanup() error {
	roots := map[string]string{
		"build":  p.BuildRoot,
		"script": p.ScriptRoot,
		"target": p.TargetRoot,
		"empty":  p.Empty,
	}

	for root, dest := range roots {
		err := os.RemoveAll(dest)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"root":  root,
			}).Error("could not remove root during cleanup")
			return err
		}
	}

	return nil
}

// Build runs the build script in the specified shell and build directory. If
// there is not a build script specified for the package, Build is basically a
// no-op but will warn about missing the script.
func (p *Package) Build() error {
	// perform the build
	buildScript, ok := p.scriptLocations["build"]
	if !ok {
		p.logger.Warn("build script not found. skipping build stage.")
		return nil
	}

	// TODO: remove the call to viper here in favor of having another piece of configuration in Package
	cmd := exec.Command(viper.GetString("shell"), buildScript)
	cmd.Dir = p.BuildRoot

	// handle out and error
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.WithField("error", err).Error("could not read command stdout")
		return err
	}
	go p.logconsumer.MustHandleStream("stdout", stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.logger.WithField("error", err).Error("could not read command stderr")
		return err
	}
	go p.logconsumer.MustHandleStream("stderr", stderr)

	err = cmd.Start()
	if err != nil {
		p.logger.WithField("error", err).Error("build script could not start")
		return err
	}

	err = cmd.Wait()
	if err != nil {
		p.logger.WithField("error", err).Error("build script exited with a non-zero exit code")
		return err
	}

	p.logger.WithFields(logrus.Fields{
		"systemTime": cmd.ProcessState.SystemTime(),
		"userTime":   cmd.ProcessState.UserTime(),
		"success":    cmd.ProcessState.Success(),
	}).Debug("build command exited")

	return nil
}

// Package drives the FPM instance created during Setup to package the output of
// the Build step.
func (p *Package) Package() error {
	for _, outType := range strings.Split(viper.GetString("type"), ",") {
		p.logger.WithField("type", outType).Info("packaging with FPM")
		out, err := p.fpm.PackageFor(outType)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error":   err,
				"out":     string(out),
				"outType": outType,
			}).Error("failed to package")
			return err
		}
	}

	return nil
}
