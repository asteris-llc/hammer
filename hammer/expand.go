package hammer

import (
	"errors"
	"fmt"
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
		grandchild.ExpandRecursive(p)
	}

	return nil
}

func (p *Package) expandSingle(child *Package) (*Package, error) {
	base := &Package{}

	// copy fields
	*base = *p

	// reset fields we should never inherit
	base.Multi = []*Package{}

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

	// single-value fields
	type Field struct {
		child interface{}
		base  interface{}
	}
	fields := []Field{
		{child.Architecture, &base.Architecture},
		{child.Depends, &base.Depends},
		{child.Description, &base.Description},
		{child.Epoch, &base.Epoch},
		{child.ExtraArgs, &base.ExtraArgs},
		{child.Iteration, &base.Iteration},
		{child.License, &base.License},
		{child.Multi, &base.Multi},
		{child.Name, &base.Name},
		{child.Resources, &base.Resources},
		{child.Scripts, &base.Scripts},
		{child.Targets, &base.Targets},
		{child.Type, &base.Type},
		{child.URL, &base.URL},
		{child.Vendor, &base.Vendor},
		{child.Version, &base.Version},
	}
	for _, field := range fields {
		switch value := field.child.(type) {
		case string: // basic values
			if value != "" {
				target, ok := field.base.(*string)
				if !ok {
					return nil, errors.New("bad value for field base")
				}
				*target = value
			}

		case []string: // dependencies
			if len(value) != 0 {
				target, ok := field.base.(*[]string)
				if !ok {
					return nil, errors.New("bad value for field base")
				}
				*target = value
			}

		case []Resource:
			if len(value) != 0 {
				target, ok := field.base.(*[]Resource)
				if !ok {
					return nil, errors.New("bad value for field base")
				}
				*target = value
			}

		case []Target:
			if len(value) != 0 {
				target, ok := field.base.(*[]Target)
				if !ok {
					return nil, errors.New("bad value for field base")
				}
				*target = value
			}

		case []*Package:
			if len(value) != 0 {
				target, ok := field.base.(*[]*Package)
				if !ok {
					return nil, errors.New("bad value for field base")
				}
				*target = value
				fmt.Println(value)
			}

		case Scripts:
			target, ok := field.base.(*Scripts)
			if !ok {
				return nil, errors.New("bad value for field base")
			}

			for name, script := range value {
				scripts[name] = script
			}

			*target = scripts

		default:
			return nil, errors.New("don't know how to handle value")
		}
	}

	return base, nil
}
