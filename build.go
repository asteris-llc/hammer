package main

import (
	"os"
	"os/signal"

	"github.com/Sirupsen/logrus"
	"github.com/asteris-llc/hammer/hammer"
	"github.com/asteris-llc/hammer/hammer/cache"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/context"
)

var (
	buildCmd = &cobra.Command{
		Use:   "build [package...]",
		Short: "build packages",
		Long:  "build all packages by default, unless specific packages are specified",
		Run: func(cmd *cobra.Command, packageNames []string) {
			loader := hammer.NewLoader(viper.GetString("search"))
			loaded, err := loader.Load()
			if err != nil {
				logrus.WithField("error", err).Fatal("could not load packages")
			}

			// find packages specified in command line arguments
			var packages []*hammer.Package
			if len(packageNames) == 0 {
				packages = loaded
			} else {
				packages = []*hammer.Package{}
				for _, name := range packageNames {
					found := false

					for _, pkg := range loaded {
						if pkg.Name == name {
							packages = append(packages, pkg)
							found = true
							break
						}
					}

					if !found {
						logrus.WithField("name", name).Warn("could not find package")
					}
				}
			}

			if len(packages) == 0 {
				logrus.Fatal("no packages selected")
			}

			// mark a single package to stream logs
			if name := viper.GetString("stream-logs-for"); name != "" {
				for _, pkg := range loaded {
					if pkg.Name == name {
						pkg.StreamLogs = true
					}
				}
			}

			// set up cache
			fsCache, err := cache.NewFSCache(viper.GetString("cache"))
			if err != nil {
				logrus.WithField("error", err).Fatal("could not make cache")
			}
			for _, pkg := range packages {
				pkg.SetCache(fsCache)
			}

			// handle interrupts so we can clean up nicely
			ctx, cancel := context.WithCancel(context.Background())

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				for {
					select {
					case <-c:
						logrus.Warn("interrupted, exiting cleanly")
						cancel()

					case <-ctx.Done():
						return
					}
				}
			}()

			// start packaging
			packager := hammer.NewPackager(packages)

			// create directories needed in the packaging process
			err = packager.EnsureOutputDir(viper.GetString("output"))
			if err != nil {
				logrus.WithField("error", err).Fatal("could not create output directory")
			}

			err = packager.EnsureOutputDir(viper.GetString("logs"))
			if err != nil {
				logrus.WithError(err).Fatal("could not create logs directory")
			}

			// build the packages!
			if !packager.Build(ctx, viper.GetInt("concurrent-jobs")) { // Errors are already reported to the user from here
				os.Exit(1)
			}
		},
	}
)
