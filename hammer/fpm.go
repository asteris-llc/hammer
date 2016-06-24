package hammer

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"

	"github.com/Sirupsen/logrus"
	shlex "github.com/anmitsu/go-shlex"
)

var (
	// ErrFieldRequired is returned when a required field is not set.
	ErrFieldRequired = errors.New("field is required")

	// ErrInvalidScriptName is returned when a bad script name is set and passed
	// to FPM.
	ErrInvalidScriptName = errors.New("invalid script name")
)

// FPM is a wrapper around the Ruby FPM tool, and will call it in a subprocess.
type FPM struct {
	Package *Package

	baseOpts []string
	baseArgs []string
}

// NewFPM does all necessary setup to run an FPM instance
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

// PackageFor runs the packaging process on a given out type ("rpm", for
// instance). It returns a string of the command combined output.
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
	var opts []string

	if len(f.Package.Targets) == 0 {
		opts = []string{
			"-s", "empty",
			"-p", f.Package.OutputRoot,
		}
	} else {
		opts = []string{
			"-s", "dir",
			"-p", f.Package.OutputRoot,
		}
	}

	type Source func() ([]string, error)
	fieldSources := []Source{
		f.baseFields,
		f.baseDependencies,
		f.baseObsoletes,
		f.baseScripts,
		f.baseConfigs,
		f.baseAttrs,
	}

	for _, source := range fieldSources {
		newOpts, err := source()
		if err != nil {
			return err
		}
		opts = append(opts, newOpts...)
	}

	// set FPM's log level to match Hammer's
	opts = append(opts, "--log")
	switch logrus.GetLevel() {
	case logrus.PanicLevel:
		fallthrough
	case logrus.FatalLevel:
		fallthrough
	case logrus.ErrorLevel:
		opts = append(opts, "error")
	case logrus.WarnLevel:
		opts = append(opts, "warn")
	case logrus.InfoLevel:
		opts = append(opts, "info")
	case logrus.DebugLevel:
		opts = append(opts, "debug")
	}

	f.baseOpts = opts
	return nil
}

func (f *FPM) baseFields() ([]string, error) {
	opts := []string{}

	type field struct {
		Name     string
		Value    string
		Required bool
	}
	fields := []field{
		{"name", f.Package.Name, true},
		{"version", f.Package.Version, true},
		{"iteration", f.Package.Iteration, true},
		{"epoch", f.Package.Epoch, false},
		{"license", f.Package.License, false},
		{"vendor", f.Package.Vendor, false},
		{"description", f.Package.Description, false},
		{"url", f.Package.URL, false},
		{"architecture", f.Package.Architecture, false},
	}

	for _, field := range fields {
		if field.Value == "" {
			if !field.Required {
				continue
			} else {
				f.Package.logger.WithField("field", field.Name).Error(ErrFieldRequired)
				return opts, ErrFieldRequired
			}
		}

		templated, err := f.Package.template.Render(field.Value)
		if err != nil {
			f.Package.logger.WithFields(logrus.Fields{
				"field": field.Name,
				"error": err,
			}).Error("failed to render field as template")
			return opts, err
		}

		opts = append(opts, "--"+field.Name, templated.String())
	}

	return opts, nil
}

func (f *FPM) baseDependencies() ([]string, error) {
	opts := []string{}

	for _, rawDepend := range f.Package.Depends {
		depend, err := f.Package.template.Render(rawDepend)
		if err != nil {
			f.Package.logger.WithFields(logrus.Fields{
				"error": err,
				"raw":   rawDepend,
			}).Error("failed to render dependency as template")
			return opts, err
		}
		opts = append(opts, "--depends", depend.String())
	}

	return opts, nil
}

func (f *FPM) baseObsoletes() ([]string, error) {
	opts := []string{}

	for _, rawObsolete := range f.Package.Obsoletes {
		obsolete, err := f.Package.template.Render(rawObsolete)
		if err != nil {
			f.Package.logger.WithFields(logrus.Fields{
				"error": err,
				"raw":   rawObsolete,
			}).Error("failed to render obsolete as template")
			return opts, err
		}
		opts = append(opts, "--replaces", obsolete.String())
	}

	return opts, nil
}

func (f *FPM) baseScripts() ([]string, error) {
	opts := []string{}

	for name, location := range f.Package.scriptLocations {
		if name == "build" {
			continue
		}

		if name != "before-install" && name != "after-install" && name != "before-remove" && name != "after-remove" && name != "before-upgrade" && name != "after-upgrade" {
			f.Package.logger.WithFields(logrus.Fields{
				"script": name,
			}).Error(ErrInvalidScriptName)
			return opts, ErrInvalidScriptName
		}

		opts = append(opts, "--"+name, location)
	}

	return opts, nil
}

func (f *FPM) baseConfigs() ([]string, error) {
	opts := []string{}

	for i, target := range f.Package.Targets {
		if !target.Config {
			continue
		}

		dest, err := f.Package.template.Render(target.Dest)
		if err != nil {
			f.Package.logger.WithField("index", i).Error("error templating target destination")
			return opts, err
		}

		opts = append(opts, "--config-files", dest.String())
	}

	return opts, nil
}

func (f *FPM) baseAttrs() ([]string, error) {
	opts := []string{}

	for _, attr := range f.Package.Attrs {
		if attr.File == "" {
			f.Package.logger.Debugf("Ignoring empty file")
			continue
		}

		// Setting to '-' has FPM use the default values
		if attr.Mode == "" {
			attr.Mode = "-"
		}
		if attr.User == "" {
			attr.User = "-"
		}
		if attr.Group == "" {
			attr.Group = "-"
		}

		opts = append(opts, "--rpm-attr", fmt.Sprintf("%s,%s,%s:%s", attr.Mode, attr.User, attr.Group, attr.File))
	}

	return opts, nil
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
