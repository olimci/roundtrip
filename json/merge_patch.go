package json

// Merge applies an RFC 7396 JSON Merge Patch to target.
//
// target and patch must be valid Nodes. The patch value is read from its owning
// Meta and cloned into target's owning Meta as needed.
func (target Node) Merge(patch Node) error {
	working := cloneMetaFromNode(target)
	root := working.Root()
	if err := root.mergePatch(patch); err != nil {
		return err
	}
	return target.replaceWithNode(root)
}

// Merge applies an RFC 7396 JSON Merge Patch to m.
//
// m and patch must be non-nil. The patch document is read but not mutated.
func (m *Meta) Merge(patch *Meta) error {
	working := cloneMetaFromNode(m.Root())
	root := working.Root()
	if err := root.mergePatch(patch.Root()); err != nil {
		return err
	}
	m.SST = working.SST
	m.Indent = working.Indent
	m.syntax = working.syntax
	return nil
}

func (target Node) mergePatch(patch Node) error {
	if patch.node.Type != NodeTypeObject {
		return target.Replace(patch)
	}
	if target.node.Type != NodeTypeObject {
		if err := target.Replace(map[string]any{}); err != nil {
			return err
		}
	}
	for name, field := range patch.ObjectFields() {
		value, _ := field.Value()
		if value.node.Type == NodeTypeNull {
			if _, ok := target.ObjectField(name); ok {
				if err := target.RemoveObjectField(name); err != nil {
					return err
				}
			}
			continue
		}
		current, ok := target.ObjectField(name)
		if ok && value.node.Type == NodeTypeObject {
			if err := current.mergePatch(value); err != nil {
				return err
			}
			continue
		}
		if ok {
			if err := current.Replace(value); err != nil {
				return err
			}
		} else if err := target.InsertObjectField(name, value); err != nil {
			return err
		}
	}
	return nil
}
