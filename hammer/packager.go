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

func (p *Packager) Build() error {
	wg := new(sync.WaitGroup)
	wg.Add(len(p.packages))

	for _, pkg := range p.packages {
		go func(pkg *Package) {
			pkg.Build()
			wg.Done()
		}(pkg)
	}

	wg.Wait()
	return nil
}
