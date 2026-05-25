package schema

// FieldIDByName resolves a field name to its stable field_id within the
// node type identified by typeName (or empty / unknown name returns
// (0, false)). This is the lookup primitive payload-translation
// (.6) calls on the ingress hot path.
//
// Lock-free: loads the current snapshot and indexes its prebuilt
// name->id maps; these are present in both frozen and runtime-mutable
// modes (publishNode builds them on every registration).
func (r *Registry) FieldIDByName(typeName, fieldName string) (uint32, bool) {
	s := r.load()
	n := s.nodesByName[typeName]
	if n == nil {
		return 0, false
	}
	if m := s.fieldNameToID[n.TypeID]; m != nil {
		id, ok := m[fieldName]
		return id, ok
	}
	if f := n.GetField(fieldName); f != nil {
		return f.FieldID, true
	}
	return 0, false
}

// FieldIDByNameForType is the type-id-keyed counterpart of
// FieldIDByName. Used by callers (ExecuteAtomic, GetNodeByKey) that
// already have a type_id in hand and want to skip the name lookup.
func (r *Registry) FieldIDByNameForType(typeID int32, fieldName string) (uint32, bool) {
	s := r.load()
	if m := s.fieldNameToID[typeID]; m != nil {
		id, ok := m[fieldName]
		return id, ok
	}
	n := s.nodes[typeID]
	if n == nil {
		return 0, false
	}
	if f := n.GetField(fieldName); f != nil {
		return f.FieldID, true
	}
	return 0, false
}

// FieldNameByID is the inverse of FieldIDByName. Egress payload
// translation calls this to rewrite stored field-id-keyed payloads
// back into name-keyed JSON for the gRPC response.
//
// Unknown ids return (empty, false) — the egress translator keeps the
// id key as-is for forward compatibility (per the schema-registry
// spec, "unknown ids on egress are kept as-is").
func (r *Registry) FieldNameByID(typeName string, fieldID uint32) (string, bool) {
	s := r.load()
	n := s.nodesByName[typeName]
	if n == nil {
		return "", false
	}
	if m := s.fieldIDToName[n.TypeID]; m != nil {
		name, ok := m[fieldID]
		return name, ok
	}
	if f := n.GetFieldByID(fieldID); f != nil {
		return f.Name, true
	}
	return "", false
}

// FieldNameByIDForType is the type-id-keyed counterpart of
// FieldNameByID.
func (r *Registry) FieldNameByIDForType(typeID int32, fieldID uint32) (string, bool) {
	s := r.load()
	if m := s.fieldIDToName[typeID]; m != nil {
		name, ok := m[fieldID]
		return name, ok
	}
	n := s.nodes[typeID]
	if n == nil {
		return "", false
	}
	if f := n.GetFieldByID(fieldID); f != nil {
		return f.Name, true
	}
	return "", false
}
