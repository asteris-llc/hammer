package hammer

import (
	"golang.org/x/net/context"
	"os"
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

type WorkerContext struct {
	packages chan *Package
	errors   chan error
	ctx      context.Context
}

func (p *Packager) startWorker(ctx *WorkerContext) {
	for {
		select {
		case pkg := <-ctx.packages:
			ctx.errors <- pkg.BuildAndPackage()

		case <-ctx.ctx.Done():
			return
		}
	}
}

func (p *Packager) Build(ctx context.Context, concurrency int) (success bool) {
	total := len(p.packages)

	workerContext := &WorkerContext{
		packages: make(chan *Package, total),
		errors:   make(chan error, total),
		ctx:      ctx,
	}

	for i := 0; i < concurrency; i++ {
		go p.startWorker(workerContext)
	}

	for _, pkg := range p.packages {
		workerContext.packages <- pkg
	}

	for i := 0; i < total; i++ {
		select {
		case err := <-workerContext.errors:
			if err != nil {
				success = false
			}

		case <-ctx.Done():
			success = false
		}
	}

	return
}
