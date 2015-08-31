package hammer

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"os/exec"
)

type Scripts map[string]string

func (s Scripts) BuildSources(p *Package, where string) (out []byte, err error) {
	build, err := p.template.Render(s["build"])
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(viper.GetString("shell"), "-e", "-c", build.String())
	cmd.Dir = where
	out, err = cmd.CombinedOutput()
	if err == nil && !cmd.ProcessState.Success() {
		err = errors.New("build command exited with a non-zero exit code")
	}

	p.logger.WithFields(logrus.Fields{
		"systemTime": cmd.ProcessState.SystemTime(),
		"userTime":   cmd.ProcessState.UserTime(),
		"success":    cmd.ProcessState.Success(),
	}).Debug("build command exited")

	return
}
