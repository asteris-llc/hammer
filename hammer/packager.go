package hammer

import (
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

func (p *Packager) Build() (success bool) {
	wg := new(sync.WaitGroup)
	wg.Add(len(p.packages))
	success = true

	for _, pkg := range p.packages {
		go func(pkg *Package) {
			// pkg.Build() is responsible for reporting errors to the user, so we just
			// need to check if there's an error and
			err := pkg.Build()
			if err != nil {
				success = false
			}
			wg.Done()
		}(pkg)
	}

	wg.Wait()
	return
}
