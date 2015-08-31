package hammer

import (
	"bytes"
	"io/ioutil"
	"path"
	"text/template"
)

type Template struct {
	Package *Package
	Funcs   template.FuncMap
}

func NewTemplate(pkg *Package) *Template {
	t := &Template{Package: pkg}

	t.Funcs = template.FuncMap{
		"include":   t.Include,
		"specFile":  t.SpecFile,
		"buildFile": t.BuildFile,
	}

	return t
}

func (t *Template) Render(in string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	tmpl, err := template.New(in).Funcs(t.Funcs).Parse(in)
	if err != nil {
		return buf, err
	}

	err = tmpl.Execute(&buf, t.Package)
	return buf, err
}

// template functions

func (t *Template) Include(path string) (string, error) {
	out, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (t *Template) SpecFile(name string) string {
	return path.Join(t.Package.SpecRoot, name)
}

func (t *Template) BuildFile(name string) string {
	return path.Join(t.Package.BuildRoot, name)
}
