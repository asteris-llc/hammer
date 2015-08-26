package hammer

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/spf13/viper"
	"os/exec"
)

type Scripts map[string]string

func (s Scripts) BuildSources(logger *logrus.Entry, where string) (out []byte, err error) {
	cmd := exec.Command(viper.GetString("shell"), "-e", "-c", s["build"])
	cmd.Dir = where
	out, err = cmd.CombinedOutput()
	if err == nil && !cmd.ProcessState.Success() {
		err = errors.New("build command exited with a non-zero exit code")
	}

	logger.WithFields(logrus.Fields{
		"systemTime": cmd.ProcessState.SystemTime(),
		"userTime":   cmd.ProcessState.UserTime(),
		"success":    cmd.ProcessState.Success(),
	}).Debug("build command exited")

	return
}
