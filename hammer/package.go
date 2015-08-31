package hammer

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

type Target struct {
	Src      string `yaml:"src"`
	Dest     string `yaml:"dest"`
	Template bool   `yaml:"template"`
	Config   bool   `yaml:"config"`
}

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
	OutputRoot string `yaml:"-"`
	ScriptRoot string `yaml:"-"`
	SpecRoot   string `yaml:"-"`
	TargetRoot string `yaml:"-"`

	// information about the machine doing the building
	CPUs int `yaml:"-"`

	logger          *logrus.Entry
	template        *Template
	scriptLocations map[string]string
	fpm             *FPM
}

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
func (p *Package) SetLogger(logger *logrus.Logger) {
	p.logger = logger.WithField("name", p.Name)
}

func (p *Package) SetTemplate(tmpl *Template) {
	p.template = tmpl
}

// process
func (p *Package) BuildAndPackage() error {
	defer p.Cleanup()
	stages := map[string]func() error{
		"setup":   p.Setup,
		"build":   p.Build,
		"package": p.Package,
	}

	for name, stage := range stages {
		logger := p.logger.WithField("stage", name)
		logger.Info("starting")
		err := stage()
		if err != nil {
			logger.WithField("error", err).Error("could not complete stage")
			return err
		}
		logger.Info("finished")
	}

	return nil
}

func (p *Package) Setup() error {
	roots := map[string]*string{
		"build":  &p.BuildRoot,
		"script": &p.ScriptRoot,
		"target": &p.TargetRoot,
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
	for _, s := range p.Resources {
		body, err := s.Download(p)
		if err != nil {
			return err
		}
		ioutil.WriteFile(
			path.Join(p.BuildRoot, s.Name(p)),
			body,
			0777,
		)
	}

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

	return nil
}

func (p *Package) Cleanup() error {
	roots := map[string]string{
		"build":  p.BuildRoot,
		"script": p.ScriptRoot,
		"target": p.TargetRoot,
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

func (p *Package) Build() error {
	// perform the build
	buildScript, ok := p.scriptLocations["build"]
	if !ok {
		p.logger.Warn("build script not found. Skipping build.")
		return nil
	}

	cmd := exec.Command(viper.GetString("shell"), buildScript)
	cmd.Dir = p.BuildRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"error":  err,
			"output": out,
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
