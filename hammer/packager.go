package hammer

import (
	"golang.org/x/net/context"
	"os"
)

// Packager takes a list of packages and controls their simultaneous building
type Packager struct {
	packages []*Package
}

// NewPackager returns a configured Package
func NewPackager(pkgs []*Package) *Packager {
	return &Packager{pkgs}
}

// EnsureOutputDir makes sure that the output directory is set
func (p *Packager) EnsureOutputDir(path string) error { // TODO: move the calling of this into "Build"
	err := os.MkdirAll(path, os.ModeDir|0777)
	if err != nil {
		return err
	}

	return nil
}

type workerContext struct {
	packages chan *Package
	errors   chan error
	ctx      context.Context
}

func (p *Packager) startWorker(ctx *workerContext) {
	for {
		select {
		case pkg := <-ctx.packages:
			ctx.errors <- pkg.BuildAndPackage()

		case <-ctx.ctx.Done():
			return
		}
	}
}

// Build builds all the packages in the Packager up to the given concurrency
// level. It assumes that the packages will report errors to the user through
// their given logger, and therefor only returns a success or failure.
func (p *Packager) Build(ctx context.Context, concurrency int) (success bool) {
	total := len(p.packages)
	success = true

	wc := &workerContext{
		packages: make(chan *Package, total),
		errors:   make(chan error, total),
		ctx:      ctx,
	}

	for i := 0; i < concurrency; i++ {
		go p.startWorker(wc)
	}

	for _, pkg := range p.packages {
		wc.packages <- pkg
	}

	for i := 0; i < total; i++ {
		select {
		case err := <-wc.errors:
			if err != nil {
				success = false
			}

		case <-ctx.Done():
			success = false
		}
	}

	return
}
