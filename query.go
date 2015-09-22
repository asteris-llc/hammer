package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/asteris-llc/hammer/hammer"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	queryCmd = &cobra.Command{
		Use:   "query {template}",
		Short: "query found packages and render information into a template",
		Long:  "provide a template to render, that will include information about the builds. The template will be rendered once per line.",
		Run: func(cmd *cobra.Command, tmpls []string) {
			if len(tmpls) != 1 {
				logrus.Fatal("please provide exactly one template")
			}

			loader := hammer.NewLoader(viper.GetString("search"))
			loaded, err := loader.Load()

			if err != nil {
				logrus.WithField("error", err).Fatal("could not load packages")
			}

			for _, pkg := range loaded {
				rendered, err := pkg.Render(tmpls[0])
				if err != nil {
					logrus.WithField("error", err).Fatal("could not render template")
				}

				fmt.Println(rendered.String())
			}
		},
	}
)
