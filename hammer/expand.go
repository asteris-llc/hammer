package hammer

import (
	"errors"
)

var (
	errBadValue     = errors.New("bad value")
	errUnknownValue = errors.New("don't know how to handle value")
)

// ExpandRecursive fills in inheritance for the Multi field
func (p *Package) ExpandRecursive(parent *Package) error {
	p.Parent = parent // should be called with nil as a parent for the top level
	p.Children = []*Package{}

	for _, sub := range p.Multi {
		grandchild, err := p.expandSingle(sub)
		if err != nil {
			return err
		}

		p.Children = append(p.Children, grandchild)
		err = grandchild.ExpandRecursive(p)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Package) expandSingle(child *Package) (*Package, error) {
	base := NewPackage()
	tmpl := base.template

	// copy fields
	*base = *p

	// reset fields we should never inherit
	base.Multi = []*Package{}
	base.template = tmpl

	scripts := Scripts{}
	for name, script := range base.Scripts {
		// we want each sub-type to have their own build script so we can share a
		// common build as much as possible.
		if name == "build" {
			continue
		}
		scripts[name] = script
	}
	base.Scripts = scripts

	// copy fields
	err := base.copyFieldsFrom(child)
	if err != nil {
		return base, err
	}

	// set stuff via methods
	base.logger = p.logger.WithField("name", base.Name)

	return base, nil
}

func (p *Package) copyFieldsFrom(other *Package) error {
	// single-value fields
	type Field struct {
		other interface{}
		p     interface{}
	}
	fields := []Field{
		{other.Architecture, &p.Architecture},
		{other.Depends, &p.Depends},
		{other.Description, &p.Description},
		{other.Epoch, &p.Epoch},
		{other.ExtraArgs, &p.ExtraArgs},
		{other.Iteration, &p.Iteration},
		{other.License, &p.License},
		{other.Multi, &p.Multi},
		{other.Name, &p.Name},
		{other.Resources, &p.Resources},
		{other.Scripts, &p.Scripts},
		{other.Targets, &p.Targets},
		{other.Type, &p.Type},
		{other.URL, &p.URL},
		{other.Vendor, &p.Vendor},
		{other.Version, &p.Version},
	}
	for _, field := range fields {
		switch value := field.other.(type) {
		case string: // basic values
			if value != "" {
				target, ok := field.p.(*string)
				if !ok {
					return errBadValue
				}
				*target = value
			}

		case []string: // dependencies
			if len(value) != 0 {
				target, ok := field.p.(*[]string)
				if !ok {
					return errBadValue
				}
				*target = value
			}

		case []Resource:
			if len(value) != 0 {
				target, ok := field.p.(*[]Resource)
				if !ok {
					return errBadValue
				}
				*target = value
			}

		case []Target:
			if len(value) != 0 {
				target, ok := field.p.(*[]Target)
				if !ok {
					return errBadValue
				}
				*target = value
			}

		case []*Package:
			if len(value) != 0 {
				target, ok := field.p.(*[]*Package)
				if !ok {
					return errBadValue
				}
				*target = value
			}

		case Scripts:
			target, ok := field.p.(*Scripts)
			if !ok {
				return errBadValue
			}

			for name, script := range value {
				(*target)[name] = script
			}

		default:
			return errUnknownValue
		}
	}

	return nil
}
