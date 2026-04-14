package entdb

// UniqueKey is a typed reference to a unique field on a node type.
//
// Instances of this struct are produced exclusively by the
// protoc-gen-entdb-keys codegen plugin. The struct fields are exported
// so the generated code can construct instances directly, but user code
// should never construct one by hand — get them from the generated
// <name>_entdb.go alongside your proto package.
//
// The T type parameter exists so [GetByKey] can constrain the value
// argument: passing a string where the generated token declares
// UniqueKey[int64] is a compile-time error. Generated code looks like:
//
//	var ProductSKU = entdb.UniqueKey[string]{TypeID: 201, FieldID: 1, Name: "sku"}
//
// The 2026-04-14 SDK v0.3 decision
// (docs/decisions/sdk_api.md) is the authority on why this type
// exists and why there is deliberately no public constructor for
// "hand-rolled" keys.
type UniqueKey[T any] struct {
	TypeID  int32
	FieldID int32
	Name    string
}
