package hammer

import (
	"errors"
	"github.com/Sirupsen/logrus"
	shlex "github.com/anmitsu/go-shlex"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
)

var (
	ErrInvalidScriptName = errors.New("invalid script name")
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

	logger   *logrus.Entry
	template *Template
}

func NewPackageFromYAML(content []byte) (*Package, error) {
	p := new(Package)
	err := yaml.Unmarshal(content, p)

	if err != nil {
		return p, err
	}

	// machine information
	p.CPUs = runtime.NumCPU()

	p.SetLogger(logrus.StandardLogger())

	return p, nil
}

func (p *Package) SetLogger(logger *logrus.Logger) {
	p.logger = logger.WithField("name", p.Name)
}

func (p *Package) SetTemplate(tmpl *Template) {
	p.template = tmpl
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
	// create temporary directory for building
	buildDir, err := ioutil.TempDir("", "hammer-"+p.Name)
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
			path.Join(buildDir, s.Name(p)),
			body,
			0777,
		)
	}

	// perform the build
	p.logger.Info("building")
	out, err := p.Scripts.BuildSources(p, buildDir)
	if err != nil {
		p.logger.WithFields(logrus.Fields{
			"error":  err,
			"output": string(out),
		}).Error("failed to build")
		return err
	}

	p.logger.Info("finished building")
	return nil
}

func (p *Package) Package() error {
	fpmArgs, err := p.fpmArgs()
	if err != nil {
		p.logger.WithField("error", err).Error("failed to get package args")
		return err
	}

	// prepend extra args
	extra, err := shlex.Split(p.ExtraArgs, true)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to parse extra args")
		return err
	}
	fpmArgs = append(extra, fpmArgs...)

	for _, outType := range strings.Split(viper.GetString("type"), ",") {
		out, err := p.FPM(fpmArgs, outType)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"out":   string(out),
			}).Error("failed to package")
			return err
		}
	}

	p.logger.Info("finished packaging")
	return nil
}

func (p *Package) FPM(args []string, pkgType string) ([]byte, error) {
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

	if fpm.ProcessState != nil {
		logger.WithFields(logrus.Fields{
			"systemTime": fpm.ProcessState.SystemTime(),
			"userTime":   fpm.ProcessState.UserTime(),
			"success":    fpm.ProcessState.Success(),
		}).Debug("package command exited")
	} else {
		logger.Debug("package command exited")
	}

	return out, err
}

func (p *Package) typeArgs(t string) []string {
	args := []string{}

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
	name, err := p.template.Render(p.Name)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render name as template")
		return args, err
	}
	args = append(args, "--name", name.String())

	// version
	version, err := p.template.Render(p.Version)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render version as template")
		return args, err
	}
	args = append(args, "--version", version.String())

	// iteration
	iteration, err := p.template.Render(p.Iteration)
	if err != nil {
		p.logger.WithField("error", err).Error("failed to render iteration as template")
		return args, err
	}
	args = append(args, "--iteration", iteration.String())

	// epoch
	if p.Epoch != "" {
		epoch, err := p.template.Render(p.Epoch)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render epoch as template")
			return args, err
		}
		args = append(args, "--epoch", epoch.String())
	}

	// license
	if p.License != "" {
		license, err := p.template.Render(p.License)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render license as template")
			return args, err
		}
		args = append(args, "--license", license.String())
	}

	// vendor
	if p.Vendor != "" {
		vendor, err := p.template.Render(p.Vendor)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render vendor as template")
			return args, err
		}
		args = append(args, "--vendor", vendor.String())
	}

	// description
	if p.Description != "" {
		description, err := p.template.Render(p.Description)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render description as template")
			return args, err
		}
		args = append(args, "--description", description.String())
	}

	// url
	if p.URL != "" {
		url, err := p.template.Render(p.URL)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render url as template")
			return args, err
		}
		args = append(args, "--url", url.String())
	}

	for _, rawDepend := range p.Depends {
		depend, err := p.template.Render(rawDepend)
		if err != nil {
			p.logger.WithFields(logrus.Fields{
				"error": err,
				"raw":   rawDepend,
			}).Error("failed to render dependency as template")
		}
		args = append(args, "--depends", depend.String())
	}

	// architecture
	if p.Architecture != "" {
		architecture, err := p.template.Render(p.Architecture)
		if err != nil {
			p.logger.WithField("error", err).Error("failed to render architecture as template")
			return args, err
		}
		args = append(args, "--architecture", architecture.String())
	}

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

		content, err := p.template.Render(value)
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

	// config files
	for i, target := range p.Targets {
		if !target.Config {
			continue
		}

		dest, err := p.template.Render(target.Dest)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target destination")
			return args, err
		}

		args = append(args, "--config-files", dest.String())
	}

	// targets
	targetDir, err := ioutil.TempDir("", "hammer-targets-"+p.Name)
	if err != nil {
		p.logger.WithField("error", err).Error("could not create target directory")
		return args, err
	}
	p.TargetRoot = targetDir

	for i, target := range p.Targets {
		srcBuf, err := p.template.Render(target.Src)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target source name")
			return args, err
		}
		src := srcBuf.String()

		dest, err := p.template.Render(target.Dest)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target destination")
			return args, err
		}

		// opt-in templating. We don't want to template *every* file because some
		// things look like Go templates and aren't (see for example every other
		// kind of mustache template)
		if !target.Template {
			args = append(args, src+"="+dest.String())
		} else {
			var content []byte
			rawContent, err := ioutil.ReadFile(src)
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"name":  src,
					"error": err,
				}).Error("error reading content")
				return args, err
			}

			contentBuf, err := p.template.Render(string(rawContent))
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"name":  src,
					"error": err,
				}).Error("error templating content")
				return args, err
			}
			content = contentBuf.Bytes()

			_, name := path.Split(src)
			contentDest := path.Join(p.TargetRoot, name)
			err = ioutil.WriteFile(contentDest, content, 0777)
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"name":  src,
					"error": err,
				}).Error("error writing content")
				return args, err
			}

			args = append(args, contentDest+"="+dest.String())
		}
	}

	return args, nil
}
