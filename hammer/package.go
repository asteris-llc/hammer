package hammer

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"
)

var (
	ErrInvalidScriptName = errors.New("invalid script name")
)

type Target struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
}

type Package struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Iteration   string     `yaml:"iteration"`
	Epoch       string     `yaml:"epoch"`
	License     string     `yaml:"license"`
	Vendor      string     `yaml:"vendor"`
	URL         string     `yaml:"url"`
	Description string     `yaml:"description"`
	Depends     []string   `yaml:"depends"`
	Resources   []Resource `yaml:"resources"`
	Targets     []Target   `yaml:"targets"`
	Scripts     Scripts    `yaml:"scripts"`

	// target-specific options // TODO: add deb, etc
	RPM map[string]string `yaml:"rpm"`

	// internal state
	BuildRoot  string `yaml:"-"`
	OutputRoot string `yaml:"-"`
	Root       string `yaml:"-"`
	ScriptRoot string `yaml:"-"`
	logger     *logrus.Entry
}

func NewPackageFromYAML(content []byte) (*Package, error) {
	p := new(Package)
	err := yaml.Unmarshal(content, p)
	p.logger = logrus.WithField("name", p.Name)
	return p, err
}

func (p *Package) Cleanup() error {
	err := os.RemoveAll(p.BuildRoot)
	if err != nil {
		p.logger.WithField("error", err).Error("could not remove build root during cleanup")
		return err
	}

	err = os.RemoveAll(p.ScriptRoot)
	if err != nil {
		p.logger.WithField("error", err).Error("could not remove script root during cleanup")
		return err
	}

	return nil
}

func (p *Package) Build() error {
	// create temporary directory for building
	buildDir, err := ioutil.TempDir("", "hammer-"+p.Name)
	defer os.Remove(buildDir)
	if err != nil {
		p.logger.WithField("error", err).Error("could not create build directory")
		return err
	}
	p.BuildRoot = buildDir

	// get the sources and store them in the temporary directory
	for _, s := range p.Resources {
		body, err := s.Download(p)
		if err != nil {
			return err
		}
		ioutil.WriteFile(
			path.Join(buildDir, s.Name()),
			body,
			0777,
		)
	}

	// perform the build
	out, err := p.Scripts.BuildSources(p.logger, buildDir)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"error":  err,
			"output": string(out),
		}).Error("failed to build")
		return err
	}

	// package the results of the build
	fpmArgs, err := p.fpmArgs()
	if err != nil {
		p.logger.WithField("error", err).Error("failed to get package args")
		return err
	}

	for _, outType := range strings.Split(viper.GetString("type"), ",") {
		out, err = p.Package(fpmArgs, outType)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"out":   string(out),
			}).Error("failed to package")
			return err
		}
	}

	return p.Cleanup()
}

func (p *Package) Render(in string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	tmpl, err := template.New(p.Name + "-" + in).Parse(in)
	if err != nil {
		return buf, err
	}

	err = tmpl.Execute(&buf, p)
	return buf, err
}

func (p *Package) Package(args []string, pkgType string) ([]byte, error) {
	logger := p.logger.WithField("type", pkgType)
	logger.Info("packaging with FPM")

	// prepend source and dest arguments
	prefixArgs := []string{
		"-s", "dir",
		"-t", pkgType,
		"-p", p.OutputRoot,
	}
	prefixArgs = append(prefixArgs, p.typeArgs(pkgType)...)
	args = append(prefixArgs, args...)

	logrus.WithField("args", args).Debug("running FPM with args")
	fpm := exec.Command("fpm", args...)
	out, err := fpm.CombinedOutput()

	if err == nil && !fpm.ProcessState.Success() {
		err = errors.New("package command exited with a non-zero exit code")
	}

	logger.WithFields(logrus.Fields{
		"systemTime": fpm.ProcessState.SystemTime(),
		"userTime":   fpm.ProcessState.UserTime(),
		"success":    fpm.ProcessState.Success(),
	}).Debug("package command exited")

	return out, err
}

func (p *Package) typeArgs(t string) []string {
	args := []string{}

	var (
		prefix string
		argMap map[string]string
	)

	switch t {
	case "rpm":
		prefix = "rpm"
		argMap = p.RPM
	}

	for key, value := range argMap {
		args = append(
			args,
			fmt.Sprintf("--%s-%s", prefix, key),
			value,
		)
	}

	// specific fixes for different output types
	switch t {
	case "rpm":
		args = append(
			args,
			"--rpm-auto-add-directories", // TODO: document this somewhere
		)
	}

	return args
}

func (p *Package) fpmArgs() ([]string, error) {
	// create the arguments for FPM
	args := []string{}

	// name
	name, err := p.Render(p.Name)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render name as template")
		return args, err
	}
	args = append(args, "--name", name.String())

	// version
	version, err := p.Render(p.Version)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render version as template")
		return args, err
	}
	args = append(args, "--version", version.String())

	// iteration
	iteration, err := p.Render(p.Iteration)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render iteration as template")
		return args, err
	}
	args = append(args, "--iteration", iteration.String())

	// epoch
	if p.Epoch != "" {
		epoch, err := p.Render(p.Epoch)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render epoch as template")
			return args, err
		}
		args = append(args, "--epoch", epoch.String())
	}

	// license
	if p.License != "" {
		license, err := p.Render(p.License)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render license as template")
			return args, err
		}
		args = append(args, "--license", license.String())
	}

	// vendor
	if p.Vendor != "" {
		vendor, err := p.Render(p.Vendor)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render vendor as template")
			return args, err
		}
		args = append(args, "--vendor", vendor.String())
	}

	// description
	if p.Description != "" {
		description, err := p.Render(p.Description)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render description as template")
			return args, err
		}
		args = append(args, "--description", description.String())
	}

	// url
	if p.URL != "" {
		url, err := p.Render(p.URL)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render url as template")
			return args, err
		}
		args = append(args, "--url", url.String())
	}

	for _, depend := range p.Depends {
		args = append(args, "--depend", depend)
	}
	// TODO: --config-files

	scriptDir, err := ioutil.TempDir("", "hammer-scripts-"+p.Name)
	if err != nil {
		p.logger.WithField("error", err).Error("could not create script directory")
		return args, err
	}
	p.ScriptRoot = scriptDir

	for name, value := range p.Scripts {
		if name == "build" {
			continue
		}
		scriptLogger := p.logger.WithField("script", name)

		// validate
		if name != "before-install" && name != "after-install" && name != "before-remove" && name != "after-remove" && name != "before-upgrade" && name != "after-upgrade" {
			scriptLogger.Error("invalid script name")
			return args, ErrInvalidScriptName
		}

		content, err := p.Render(value)
		if err != nil {
			scriptLogger.WithField("error", err).Error("error in templating script")
			return args, err
		}

		// write
		loc := path.Join(scriptDir, name)
		err = ioutil.WriteFile(loc, content.Bytes(), 0755)
		if err != nil {
			scriptLogger.WithField("error", err).Error("could not write script")
			return args, err
		}

		scriptLogger.Debug("wrote script")
		args = append(args, "--"+name, loc)
	}

	// TODO: --config-files

	for i, target := range p.Targets {
		// TODO: template source content
		src, err := p.Render(target.Src)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target source name")
			return args, err
		}

		dest, err := p.Render(target.Dest)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target destination")
			return args, err
		}

		args = append(args, src.String()+"="+dest.String())
	}

	return args, nil
}
