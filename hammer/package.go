package hammer

import (
	"bytes"
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
	Architecture string     `yaml:"architecture,omitempty"`
	Depends      []string   `yaml:"depends,omitempty"`
	Description  string     `yaml:"description,omitempty"`
	Epoch        string     `yaml:"epoch,omitempty"`
	ExtraArgs    string     `yaml:"extra-args,omitempty"`
	Iteration    string     `yaml:"iteration,omitempty"`
	License      string     `yaml:"license,omitempty"`
	Name         string     `yaml:"name,omitempty"`
	Resources    []Resource `yaml:"resources,omitempty"`
	Scripts      Scripts    `yaml:"scripts,omitempty"`
	Targets      []Target   `yaml:"targets,omitempty"`
	Type         string     `yaml:"type,omitempty"`
	URL          string     `yaml:"url,omitempty"`
	Vendor       string     `yaml:"vendor,omitempty"`
	Version      string     `yaml:"version,omitempty"`

	// Multi parametrizes builds by expanding recursively. This information is
	// then moved to Parent and Children.
	Multi []*Package `yaml:"multi,omitempty"`

	// various roots
	BuildRoot  string `yaml:"-"`
	Empty      string `yaml:"-"`
	OutputRoot string `yaml:"-"`
	ScriptRoot string `yaml:"-"`
	SpecRoot   string `yaml:"-"`
	TargetRoot string `yaml:"-"`
	LogRoot    string `yaml:"-"`

	// graph of builds
	Parent   *Package   `yaml:"-"`
	Children []*Package `yaml:"-"`

	// information about the machine doing the building
	CPUs int `yaml:"-"`

	cache           cache.Cache
	fpm             *FPM
	logconsumer     LogConsumer
	logger          *logrus.Entry
	scriptLocations map[string]string
	template        *Template
}

// NewPackage sets up defaults for build machine information, the logger, and
// the templater
func NewPackage() *Package {
	p := new(Package)

	// machine information
	p.CPUs = runtime.NumCPU()

	// private fields
	p.SetLogger(logrus.StandardLogger())
	p.SetTemplate(NewTemplate(p))

	return p
}

// NewPackageFromYAML loads a package from YAML if it can
func NewPackageFromYAML(content []byte) (*Package, error) {
	p := NewPackage()
	err := yaml.Unmarshal(content, p)

	if err != nil {
		return p, err
	}

	// now that name is set, set logger...
	p.logger = p.logger.WithField("name", p.Name)

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

// Render maybe renders a template string in the context of this package.
func (p *Package) Render(tmpl string) (bytes.Buffer, error) {
	return p.template.Render(tmpl)
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
		stdout, err := p.logconsumer.Replay("stdout")
		if err != nil {
			p.logger.WithField("error", err).Error("build script exited and could not read stdout")
		}

		stderr, err := p.logconsumer.Replay("stderr")
		if err != nil {
			p.logger.WithField("error", err).Error("build script exited and could not read stdout")
		}

		p.logger.WithFields(logrus.Fields{
			"error":  err,
			"stdout": string(stdout),
			"stderr": string(stderr),
		}).Error("build script exited with a non-zero exit code")
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
	out, err := p.fpm.PackageFor(p.Type)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"error":   err,
			"out":     string(out),
			"outType": p.Type,
		}).Error("failed to package")
		return err
	}

	return nil
}

// TotalPackages is the total count of packages for this and all children.
func (p *Package) TotalPackages() int {
	count := 1
	for _, pkg := range p.Children {
		count += pkg.TotalPackages()
	}
	return count
}
