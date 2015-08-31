package hammer

import (
	"bytes"
	"text/template"
)

type Template struct {
	Package *Package
	Funcs   template.FuncMap
}

func NewTemplate(pkg *Package) *Template {
	return &Template{
		Package: pkg,
		Funcs:   template.FuncMap{},
	}
}

func (t *Template) Render(in string) (bytes.Buffer, error) {
	var buf bytes.Buffer
	tmpl, err := template.New(in).Parse(in)
	if err != nil {
		return buf, err
	}

	err = tmpl.Funcs(t.Funcs).Execute(&buf, t.Package)
	return buf, err
}
