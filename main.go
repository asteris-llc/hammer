package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/asteris-llc/hammer/hammer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path"
	"strings"
)

var (
	RootCmd = &cobra.Command{
		Use:   "hammer",
		Short: "hammer builds a bunch of package specs at once",
		Run: func(cmd *cobra.Command, args []string) {
			loader := hammer.NewLoader(viper.GetString("search"))
			packages, err := loader.Load()
			if err != nil {
				logrus.WithField("error", err).Fatal("could not load packages")
			}

			packager := hammer.NewPackager(packages)

			err = packager.EnsureOutputDir(viper.GetString("output"))
			if err != nil {
				logrus.WithField("error", err).Fatal("could not create output directory")
			}

			only := viper.GetString("only")
			if only != "" {
				packager.Only(strings.Split(only, ","))
			}

			exclude := viper.GetString("exclude")
			if exclude != "" {
				packager.Exclude(strings.Split(exclude, ","))
			}

			if !packager.Build() { // Errors are already reported to the user from here
				os.Exit(1)
			}
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
	}
)

func init() {
	// set defaults
	RootCmd.PersistentFlags().String("log-level", "info", "one of debug, info, warn, error, or fatal")
	RootCmd.PersistentFlags().String("log-format", "text", "specify output (text or json)")
	RootCmd.PersistentFlags().String("shell", "bash", "shell to use for executing build scripts")
	RootCmd.PersistentFlags().String("type", "rpm", "type of package to build (flag can be repeated)")
	RootCmd.PersistentFlags().String("only", "", "only build named packages")
	RootCmd.PersistentFlags().String("exclude", "", "exclude named packages")

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithField("error", err).Warning("could not get working directory")
	}
	RootCmd.PersistentFlags().String("search", cwd, "where to look for package definitions")
	RootCmd.PersistentFlags().String("output", path.Join(cwd, "out"), "where to place output packages")

	err = viper.BindPFlags(RootCmd.PersistentFlags())
	if err != nil {
		logrus.WithField("error", err).Error("could not bind flags")
		os.Exit(1)
	}
}

func setupLogging() {
	switch viper.GetString("log-level") {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	default:
		logrus.WithField("log-level", viper.GetString("log-level")).Warning("invalid log level. defaulting to info.")
		logrus.SetLevel(logrus.InfoLevel)
	}

	switch viper.GetString("log-format") {
	case "text":
		logrus.SetFormatter(new(logrus.TextFormatter))
	case "json":
		logrus.SetFormatter(new(logrus.JSONFormatter))
	default:
		logrus.WithField("log-format", viper.GetString("log-format")).Warning("invalid log format. defaulting to text.")
		logrus.SetFormatter(new(logrus.TextFormatter))
	}
}

func main() {
	RootCmd.Execute()
}
