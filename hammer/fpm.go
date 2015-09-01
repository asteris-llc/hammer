package hammer

import (
	"errors"
	"github.com/Sirupsen/logrus"
	shlex "github.com/anmitsu/go-shlex"
	"io/ioutil"
	"os/exec"
	"path"
)

var (
	ErrFieldRequired     = errors.New("field is required")
	ErrInvalidScriptName = errors.New("invalid script name")
)

type FPM struct {
	Package *Package

	baseOpts []string
	baseArgs []string
}

func NewFPM(p *Package) (*FPM, error) {
	fpm := &FPM{
		Package: p,
	}

	err := fpm.setBaseArgs()
	if err != nil {
		return nil, err
	}

	err = fpm.setBaseOpts()
	if err != nil {
		return nil, err
	}

	return fpm, nil
}

func (f *FPM) PackageFor(outType string) (string, error) {
	// put args and opts all together
	extra, err := f.extraArgs()
	if err != nil {
		return "", err
	}

	arguments := []string{}
	arguments = append(arguments, f.baseOpts...)
	arguments = append(arguments, f.optsForType(outType)...)
	arguments = append(arguments, extra...)
	arguments = append(arguments, f.baseArgs...)

	f.Package.logger.WithField("args", arguments).Debug("running FPM with args")
	fpm := exec.Command("fpm", arguments...)
	out, err := fpm.CombinedOutput()

	if fpm.ProcessState != nil {
		f.Package.logger.WithFields(logrus.Fields{
			"systemTime": fpm.ProcessState.SystemTime(),
			"userTime":   fpm.ProcessState.UserTime(),
			"success":    fpm.ProcessState.Success(),
		}).Debug("package command exited")
	} else {
		f.Package.logger.Debug("package command exited")
	}

	return string(out), err
}

func (f *FPM) setBaseArgs() error {
	args := []string{}

	p := f.Package

	// targets
	for i, target := range p.Targets {
		srcBuf, err := p.template.Render(target.Src)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target source name")
			return err
		}
		src := srcBuf.String()

		dest, err := p.template.Render(target.Dest)
		if err != nil {
			p.logger.WithField("index", i).Error("error templating target destination")
			return err
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
				return err
			}

			contentBuf, err := p.template.Render(string(rawContent))
			if err != nil {
				p.logger.WithFields(logrus.Fields{
					"name":  src,
					"error": err,
				}).Error("error templating content")
				return err
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
				return err
			}

			args = append(args, contentDest+"="+dest.String())
		}
	}

	f.baseArgs = args
	return nil
}

func (f *FPM) setBaseOpts() error {
	opts := []string{
		"-s", "dir",
		"-p", f.Package.OutputRoot,
	}

	pkg := f.Package

	// fields
	type field struct {
		Name     string
		Value    string
		Required bool
	}
	fields := []field{
		{"name", pkg.Name, true},
		{"version", pkg.Version, true},
		{"iteration", pkg.Iteration, true},
		{"epoch", pkg.Epoch, false},
		{"license", pkg.License, false},
		{"vendor", pkg.Vendor, false},
		{"description", pkg.Description, false},
		{"url", pkg.URL, false},
		{"architecture", pkg.Architecture, false},
	}

	for _, field := range fields {
		if field.Value == "" {
			if field.Required {
				pkg.logger.WithField("field", field.Name).Error(ErrFieldRequired)
				return ErrFieldRequired
			} else {
				continue
			}
		}

		templated, err := pkg.template.Render(field.Value)
		if err != nil {
			pkg.logger.WithFields(logrus.Fields{
				"field": field.Name,
				"error": err,
			}).Error("failed to render field as template")
			return err
		}

		opts = append(opts, "--"+field.Name, templated.String())
	}

	// dependencies
	for _, rawDepend := range pkg.Depends {
		depend, err := pkg.template.Render(rawDepend)
		if err != nil {
			pkg.logger.WithFields(logrus.Fields{
				"error": err,
				"raw":   rawDepend,
			}).Error("failed to render dependency as template")
		}
		opts = append(opts, "--depends", depend.String())
	}

	// scripts
	for name, location := range pkg.scriptLocations {
		if name == "build" {
			continue
		}

		if name != "before-install" && name != "after-install" && name != "before-remove" && name != "after-remove" && name != "before-upgrade" && name != "after-upgrade" {
			pkg.logger.WithFields(logrus.Fields{
				"script": name,
			}).Error(ErrInvalidScriptName)
		}

		opts = append(opts, "--"+name, location)
	}

	// config files
	for i, target := range pkg.Targets {
		if !target.Config {
			continue
		}

		dest, err := pkg.template.Render(target.Dest)
		if err != nil {
			pkg.logger.WithField("index", i).Error("error templating target destination")
			return err
		}

		opts = append(opts, "--config-files", dest.String())
	}

	f.baseOpts = opts
	return nil
}

func (f *FPM) optsForType(t string) []string {
	opts := []string{
		"-t", t,
	}

	return opts
}

func (f *FPM) extraArgs() ([]string, error) {
	extra, err := shlex.Split(f.Package.ExtraArgs, true)
	if err != nil {
		f.Package.logger.WithField("error", err).Error("failed to parse extra args")
	}
	return extra, err
}
