package entdb

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// nodeFromProto converts the wire-level *pb.Node into the SDK's
// public Node struct. The payload is carried as a
// google.protobuf.Struct on the wire and expanded into a
// map[string]any keyed by the proto field id (as decimal string),
// matching the "field IDs, not field names, on disk" invariant
// spelled out in CLAUDE.md.
func nodeFromProto(n *pb.Node) *Node {
	if n == nil {
		return nil
	}
	owner, _ := ParseActor(n.GetOwnerActor())
	return &Node{
		TenantID:   n.GetTenantId(),
		NodeID:     n.GetNodeId(),
		TypeID:     int(n.GetTypeId()),
		Payload:    structToMap(n.GetPayload()),
		CreatedAt:  n.GetCreatedAt(),
		UpdatedAt:  n.GetUpdatedAt(),
		OwnerActor: owner,
		ACL:        aclFromProto(n.GetAcl()),
	}
}

// edgeFromProto converts the wire-level *pb.Edge into the SDK's
// public Edge struct.
func edgeFromProto(e *pb.Edge) *Edge {
	if e == nil {
		return nil
	}
	return &Edge{
		TenantID:   e.GetTenantId(),
		EdgeTypeID: int(e.GetEdgeTypeId()),
		FromNodeID: e.GetFromNodeId(),
		ToNodeID:   e.GetToNodeId(),
		Props:      structToMap(e.GetProps()),
		CreatedAt:  e.GetCreatedAt(),
	}
}

// aclFromProto converts a list of *pb.AclEntry into the SDK's
// typed []ACLEntry. The server emits both the legacy “principal“
// field and the 2026-04-13 “grantee“ field — we prefer the
// latter when present, falling back to “principal“ for wire
// compatibility during the migration window.
func aclFromProto(entries []*pb.AclEntry) []ACLEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]ACLEntry, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		granteeStr := e.GetGrantee()
		if granteeStr == "" {
			granteeStr = e.GetPrincipal()
		}
		grantee, _ := ParseActor(granteeStr)
		out = append(out, ACLEntry{
			Grantee:    grantee,
			Permission: Permission(e.GetPermission()),
			ExpiresAt:  e.GetExpiresAt(),
		})
	}
	return out
}

// structToMap converts a *structpb.Struct into the SDK's
// map[string]any payload form. A nil input yields nil — callers
// treat "no payload" and "empty payload" as the same thing.
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil || len(s.GetFields()) == 0 {
		return nil
	}
	return s.AsMap()
}

// mapToStruct builds a *structpb.Struct from a map[string]any.
// Returns nil on empty input so callers can pass nil through to
// the wire without constructing an empty Struct.
func mapToStruct(m map[string]any) (*structpb.Struct, error) {
	if len(m) == 0 {
		return nil, nil
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		return nil, fmt.Errorf("entdb: encode struct: %w", err)
	}
	return s, nil
}

// aclToProto converts the SDK's []ACLEntry into the wire
// []*pb.AclEntry form. Populates both “principal“ and
// “grantee“ for wire compatibility with older servers while
// the 2026-04-13 migration completes.
func aclToProto(entries []ACLEntry) []*pb.AclEntry {
	if len(entries) == 0 {
		return nil
	}
	out := make([]*pb.AclEntry, 0, len(entries))
	for _, e := range entries {
		s := e.Grantee.String()
		out = append(out, &pb.AclEntry{
			Principal:  s,
			Grantee:    s,
			Permission: string(e.Permission),
			ExpiresAt:  e.ExpiresAt,
		})
	}
	return out
}

// storageModeToProto converts the SDK's StorageMode to the wire
// enum.
func storageModeToProto(m StorageMode) pb.StorageMode {
	switch m {
	case StorageModeUserMailbox:
		return pb.StorageMode_STORAGE_MODE_USER_MAILBOX
	case StorageModePublic:
		return pb.StorageMode_STORAGE_MODE_PUBLIC
	default:
		return pb.StorageMode_STORAGE_MODE_TENANT
	}
}

// operationsToProto converts the SDK's []Operation into the wire
// []*pb.Operation. The SDK-side Operation is a discriminated union
// tagged by OperationType; we fan it out into the protobuf oneof
// wrapper types.
func operationsToProto(ops []Operation) ([]*pb.Operation, error) {
	out := make([]*pb.Operation, 0, len(ops))
	for _, op := range ops {
		po := &pb.Operation{}
		switch op.Type {
		case OpCreateNode:
			data, err := mapToStruct(op.Data)
			if err != nil {
				return nil, err
			}
			po.Op = &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId:       int32(op.TypeID),
				Id:           op.NodeID,
				As:           op.Alias,
				Data:         data,
				Acl:          aclToProto(op.ACL),
				StorageMode:  storageModeToProto(op.StorageMode),
				TargetUserId: op.TargetUserID,
			}}
		case OpUpdateNode:
			patch, err := mapToStruct(op.Patch)
			if err != nil {
				return nil, err
			}
			po.Op = &pb.Operation_UpdateNode{UpdateNode: &pb.UpdateNodeOp{
				TypeId: int32(op.TypeID),
				Id:     op.NodeID,
				Patch:  patch,
			}}
		case OpDeleteNode:
			po.Op = &pb.Operation_DeleteNode{DeleteNode: &pb.DeleteNodeOp{
				TypeId: int32(op.TypeID),
				Id:     op.NodeID,
			}}
		case OpCreateEdge:
			po.Op = &pb.Operation_CreateEdge{CreateEdge: &pb.CreateEdgeOp{
				EdgeId: int32(op.EdgeTypeID),
				From:   &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: op.FromNodeID}},
				To:     &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: op.ToNodeID}},
			}}
		case OpDeleteEdge:
			po.Op = &pb.Operation_DeleteEdge{DeleteEdge: &pb.DeleteEdgeOp{
				EdgeId: int32(op.EdgeTypeID),
				From:   &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: op.FromNodeID}},
				To:     &pb.NodeRef{Ref: &pb.NodeRef_Id{Id: op.ToNodeID}},
			}}
		default:
			return nil, fmt.Errorf("entdb: unknown operation type %v", op.Type)
		}
		out = append(out, po)
	}
	return out, nil
}

// filterToProto converts the SDK's MongoDB-style filter map into
// a slice of *pb.FieldFilter. The mapping matches the Python
// SDK's behaviour: top-level entries become equality filters; a
// nested map with “$<op>“ keys emits one FieldFilter per operator
// and selects the appropriate FilterOp.
//
// Unknown operators (e.g. “$nin“, “$between“, “$and“) are not
// expressible in the wire's FilterOp enum, so they are passed
// through as the raw subtree on a single EQ filter — the server
// reconstructs a MongoDB-style operator dict from that and runs it
// through the SQL compiler, where unknown operators surface as a
// typed ValidationError. To keep this fallback deterministic
// regardless of Go's randomised map iteration, we detect the
// "contains an unknown operator" case up-front and emit a single
// pass-through filter rather than mixing per-op filters with a
// pass-through.
func filterToProto(filter map[string]any) ([]*pb.FieldFilter, error) {
	if len(filter) == 0 {
		return nil, nil
	}
	out := make([]*pb.FieldFilter, 0, len(filter))
	for field, raw := range filter {
		// Nested operator dict — e.g. {"price": {"$gte": 100}}
		if sub, ok := raw.(map[string]any); ok && len(sub) > 0 {
			if subHasUnknownOp(sub) {
				// Pass the full subtree through as a Struct on a
				// single EQ filter — the server's
				// _field_filters_to_filter_dict reconstructs the
				// operator dict and the SQL compiler validates it.
				v, err := structpb.NewValue(raw)
				if err != nil {
					return nil, fmt.Errorf("entdb: filter field %q: %w", field, err)
				}
				out = append(out, &pb.FieldFilter{Field: field, Op: pb.FilterOp_EQ, Value: v})
				continue
			}
			for opKey, val := range sub {
				op, _ := filterOpFromKey(opKey)
				v, err := structpb.NewValue(val)
				if err != nil {
					return nil, fmt.Errorf("entdb: filter field %q op %q: %w", field, opKey, err)
				}
				out = append(out, &pb.FieldFilter{Field: field, Op: op, Value: v})
			}
			continue
		}
		// Plain equality — e.g. {"sku": "WIDGET-1"}
		v, err := structpb.NewValue(raw)
		if err != nil {
			return nil, fmt.Errorf("entdb: filter field %q: %w", field, err)
		}
		out = append(out, &pb.FieldFilter{Field: field, Op: pb.FilterOp_EQ, Value: v})
	}
	return out, nil
}

// subHasUnknownOp reports whether any key in the operator dict is
// not representable on the wire as a FilterOp enum value.
func subHasUnknownOp(sub map[string]any) bool {
	for k := range sub {
		if _, ok := filterOpFromKey(k); !ok {
			return true
		}
	}
	return false
}

// filterOpFromKey maps the MongoDB-style "$eq"/"$gte"/... strings
// to the wire enum. Returns (_, false) for unknown operators so
// the caller can fall back to a raw-value filter or raise a
// validation error.
func filterOpFromKey(key string) (pb.FilterOp, bool) {
	switch key {
	case "$eq":
		return pb.FilterOp_EQ, true
	case "$ne":
		return pb.FilterOp_NEQ, true
	case "$gt":
		return pb.FilterOp_GT, true
	case "$gte":
		return pb.FilterOp_GTE, true
	case "$lt":
		return pb.FilterOp_LT, true
	case "$lte":
		return pb.FilterOp_LTE, true
	case "$contains":
		return pb.FilterOp_CONTAINS, true
	case "$in":
		return pb.FilterOp_IN, true
	}
	return pb.FilterOp_EQ, false
}

// translateGRPCError converts a gRPC status error into one of the
// SDK's typed error structs. The mapping mirrors the Python SDK
// so callers can write a single “errors.As“ chain that covers
// both languages.
//
// Priority:
//  1. AlreadyExists → *UniqueConstraintError (with structured
//     coordinates when the server emits them in the status
//     message).
//  2. ResourceExhausted → *RateLimitError (with retry-after hint
//     from trailing metadata if available).
//  3. NotFound → *NotFoundError.
//  4. PermissionDenied / Unauthenticated → *AccessDeniedError.
//  5. InvalidArgument → *ValidationError.
//  6. Unavailable / DeadlineExceeded → *ConnectionError.
//  7. default → *EntDBError carrying the raw code + message.
//
// “tenantID“ is threaded through so the unique-constraint error
// can carry the tenant it was raised against; it is ignored for
// all other mappings.
func translateGRPCError(err error, tenantID, address string) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &EntDBError{Message: err.Error(), Code: "UNKNOWN"}
	}
	return translateGRPCStatus(st, nil, tenantID, address)
}

// translateGRPCStatusWithTrailer is the trailer-aware variant of
// translateGRPCError. The trailing-metadata split matters for
// rate-limit responses, which carry the “retry-after“ hint only
// on the trailer (not the status message).
func translateGRPCStatusWithTrailer(err error, trailer metadata.MD, tenantID, address string) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return &EntDBError{Message: err.Error(), Code: "UNKNOWN"}
	}
	return translateGRPCStatus(st, trailer, tenantID, address)
}

func translateGRPCStatus(st *status.Status, trailer metadata.MD, tenantID, address string) error {
	switch st.Code() {
	case codes.OK:
		return nil
	case codes.AlreadyExists:
		if uce := parseUniqueConstraintFromStatus(st.Err(), tenantID); uce != nil {
			return uce
		}
		return &EntDBError{Message: st.Message(), Code: "ALREADY_EXISTS"}
	case codes.ResourceExhausted:
		if rle := parseRateLimitFromStatus(st.Err(), trailer); rle != nil {
			return rle
		}
		return NewRateLimitError(st.Message(), 0)
	case codes.NotFound:
		return &NotFoundError{
			EntDBError: EntDBError{
				Message: st.Message(),
				Code:    "NOT_FOUND",
				Details: map[string]any{"message": st.Message()},
			},
		}
	case codes.PermissionDenied, codes.Unauthenticated:
		return &AccessDeniedError{
			EntDBError: EntDBError{
				Message: st.Message(),
				Code:    "ACCESS_DENIED",
				Details: map[string]any{"message": st.Message()},
			},
		}
	case codes.InvalidArgument:
		return &ValidationError{
			EntDBError: EntDBError{
				Message: st.Message(),
				Code:    "VALIDATION_ERROR",
				Details: map[string]any{"message": st.Message()},
			},
		}
	case codes.Unavailable, codes.DeadlineExceeded:
		return NewConnectionError(st.Message(), address)
	default:
		return &EntDBError{
			Message: st.Message(),
			Code:    st.Code().String(),
		}
	}
}
