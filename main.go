package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path"
	"runtime"
)

var (
	RootCmd = &cobra.Command{
		Use:   "hammer",
		Short: "hammer builds a bunch of package specs at once",
		Run: func(cmd *cobra.Command, args []string) {
			logrus.Fatal("no command specified (try `hammer help build`)")
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
	}
)

func init() {
	// root and persistent flags
	RootCmd.PersistentFlags().String("log-level", "info", "one of debug, info, warn, error, or fatal")
	RootCmd.PersistentFlags().String("log-format", "text", "specify output (text or json)")

	// build flags
	BuildCmd.Flags().String("shell", "bash", "shell to use for executing build scripts")
	BuildCmd.Flags().String("type", "rpm", "type of package to build (multiple build targets should be separated by commas)")
	BuildCmd.Flags().Int("concurrent-jobs", runtime.NumCPU(), "number of packages to build at once")

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithField("error", err).Warning("could not get working directory")
	}
	BuildCmd.Flags().String("search", cwd, "where to look for package definitions")
	BuildCmd.Flags().String("output", path.Join(cwd, "out"), "where to place output packages")
	BuildCmd.Flags().String("logs", path.Join(cwd, "logs"), "where to place build logs")

	viper.BindPFlags(RootCmd.PersistentFlags())
	viper.BindPFlags(BuildCmd.Flags())
}

func main() {
	RootCmd.AddCommand(BuildCmd)
	RootCmd.Execute()
}
