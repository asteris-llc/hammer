package hammer

import (
	"bytes"
	"io/ioutil"
	"path"
	"text/template"
)

// Template controls template rendering within a Package
type Template struct {
	Package *Package
	Funcs   template.FuncMap
}

// NewTemplate takes a Package and returns a configured Template.
func NewTemplate(pkg *Package) *Template {
	t := &Template{Package: pkg}

	t.Funcs = template.FuncMap{
		"buildFile":       t.BuildFile,
		"empty":           t.Empty,
		"include":         t.Include,
		"includeTemplate": t.IncludeTemplate,
		"specFile":        t.SpecFile,
	}

	return t
}

// Render renders the input string according to the template rules and returns a
// buffer. This can be read with `bytes.Buffer.String()` or
// `bytes.Buffer.Bytes()`
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

// Include (include) is a template function that returns the raw version of a
// file
func (t *Template) Include(path string) (string, error) {
	out, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// IncludeTemplate (includeTemplate) is a template function that renders a file
// from disk and returns the output
func (t *Template) IncludeTemplate(path string) (string, error) {
	tmpl, err := t.Include(path)
	if err != nil {
		return "", err
	}

	rendered, err := t.Render(tmpl)
	return rendered.String(), err
}

// SpecFile (specFile) is a template function that joins the SpecRoot with the
// given name. Useful in target specifications.
func (t *Template) SpecFile(name string) string {
	return path.Join(t.Package.SpecRoot, name)
}

// BuildFile (buildFile) is a template function that joins the SpecRoot with the
// given name. Useful in target specifications.
func (t *Template) BuildFile(name string) string {
	return path.Join(t.Package.BuildRoot, name)
}

// Empty (empty) is a template function that just returns the path to an empty
// directory. This is useful for creating empty directories in targets (like
// `/etc/yourpackage/conf.d`)
func (t *Template) Empty() string {
	return t.Package.Empty + "/"
}
