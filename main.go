package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
	"path"
	"runtime"
)

var (
	rootCmd = &cobra.Command{
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
	rootCmd.PersistentFlags().String("log-level", "info", "one of debug, info, warn, error, or fatal")
	rootCmd.PersistentFlags().String("log-format", "text", "specify output (text or json)")

	// build flags
	buildCmd.Flags().String("shell", "bash", "shell to use for executing build scripts")
	buildCmd.Flags().String("type", "rpm", "type of package to build (multiple build targets should be separated by commas)")
	buildCmd.Flags().Int("concurrent-jobs", runtime.NumCPU(), "number of packages to build at once")

	cwd, err := os.Getwd()
	if err != nil {
		logrus.WithField("error", err).Warning("could not get working directory")
	}
	buildCmd.Flags().String("search", cwd, "where to look for package definitions")
	buildCmd.Flags().String("output", path.Join(cwd, "out"), "where to place output packages")
	buildCmd.Flags().String("logs", path.Join(cwd, "logs"), "where to place build logs")
	buildCmd.Flags().String("cache", path.Join(cwd, ".hammer-cache"), "where to cache downloads")

	for _, flags := range []*pflag.FlagSet{rootCmd.PersistentFlags(), buildCmd.Flags()} {
		err := viper.BindPFlags(flags)
		if err != nil {
			logrus.WithField("error", err).Fatal("could not bind flags")
		}
	}
}

func main() {
	rootCmd.AddCommand(buildCmd)
	err := rootCmd.Execute()
	if err != nil {
		logrus.WithField("error", err).Fatal("exited with error")
	}
}
