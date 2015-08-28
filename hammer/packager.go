package hammer

import (
	"github.com/Sirupsen/logrus"
	"os"
	"sync"
)

type Packager struct {
	packages []*Package
}

func NewPackager(pkgs []*Package) *Packager {
	return &Packager{pkgs}
}

func (p *Packager) EnsureOutputDir(path string) error {
	err := os.MkdirAll(path, os.ModeDir|0777)
	if err != nil {
		return err
	}

	return nil
}

func (p *Packager) Only(names []string) {
	tmp := []*Package{}
	for _, pkg := range p.packages {
		found := false
		for _, name := range names {
			if pkg.Name == name {
				found = true
				break
			}
		}

		if found {
			tmp = append(tmp, pkg)
		} else {
			logrus.WithField("name", pkg.Name).Info("skipping build")
		}
	}

	p.packages = tmp
}

func (p *Packager) Exclude(names []string) {
	tmp := []*Package{}
	for _, pkg := range p.packages {
		found := false
		for _, name := range names {
			if pkg.Name == name {
				found = true
				break
			}
		}

		if !found {
			tmp = append(tmp, pkg)
		} else {
			logrus.WithField("name", pkg.Name).Info("skipping build")
		}
	}

	p.packages = tmp
}

func (p *Packager) Build() (success bool) {
	wg := new(sync.WaitGroup)
	wg.Add(len(p.packages))
	success = true

	for _, pkg := range p.packages {
		go func(pkg *Package) {
			// these functions are responsible for reporting errors to the user, so we
			// just need to check if there's an error and set the success value
			err := pkg.Build()
			if err != nil {
				success = false
			}

			err = pkg.Package()
			if err != nil {
				success = false
			}

			err = pkg.Cleanup()
			if err != nil {
				success = false
			}

			wg.Done()
		}(pkg)
	}

	wg.Wait()
	return
}
