// SPDX-License-Identifier: AGPL-3.0-only

package acl

import (
	"context"
	"fmt"
)

// Enforcer is the top-level ACL facade. One Enforcer per server; it
// owns the Registry, Resolver, Checker and Filter. Mirrors the role
// of AclManager + CapabilityRegistry + canonical_store ACL helpers in
// the Python server.
//
// Handlers in server/go/internal/api do not call Resolver / Checker /
// Filter directly — they go through Enforcer so the (registry,
// resolver, readers) wiring stays in one place and is exchangable in
// tests.
type Enforcer struct {
	registry *Registry
	resolver *Resolver
	checker  *Checker
	filter   *Filter
}

// EnforcerOptions wires an Enforcer. Registry, Resolver, Checker and
// Filter are all required. Any nil field returns an error.
type EnforcerOptions struct {
	Registry *Registry
	Resolver *Resolver
	Checker  *Checker
	Filter   *Filter
}

// NewEnforcer constructs an Enforcer.
func NewEnforcer(opts EnforcerOptions) (*Enforcer, error) {
	if opts.Registry == nil {
		return nil, fmt.Errorf("acl: NewEnforcer: Registry required")
	}
	if opts.Resolver == nil {
		return nil, fmt.Errorf("acl: NewEnforcer: Resolver required")
	}
	if opts.Checker == nil {
		return nil, fmt.Errorf("acl: NewEnforcer: Checker required")
	}
	if opts.Filter == nil {
		return nil, fmt.Errorf("acl: NewEnforcer: Filter required")
	}
	return &Enforcer{
		registry: opts.Registry,
		resolver: opts.Resolver,
		checker:  opts.Checker,
		filter:   opts.Filter,
	}, nil
}

// Registry returns the registry the Enforcer was built with.
func (e *Enforcer) Registry() *Registry { return e.registry }

// Resolver returns the resolver the Enforcer was built with.
func (e *Enforcer) Resolver() *Resolver { return e.resolver }

// Check delegates to the Checker. See CheckRequest for the input shape.
func (e *Enforcer) Check(ctx context.Context, req CheckRequest) error {
	return e.checker.Check(ctx, req)
}

// FilterReadable delegates to the Filter.
func (e *Enforcer) FilterReadable(ctx context.Context, tenantID string, a Actor, nodeIDs []string) ([]string, error) {
	return e.filter.FilterReadable(ctx, tenantID, a, nodeIDs)
}

// Expand delegates to the Resolver.
func (e *Enforcer) Expand(ctx context.Context, tenantID string, a Actor) ([]Actor, error) {
	return e.resolver.Expand(ctx, tenantID, a)
}

// RequiredForOp delegates to the Registry.
func (e *Enforcer) RequiredForOp(typeID int32, op string, q OpQuery) (CoreCapability, ExtCapID) {
	return e.registry.RequiredForOp(typeID, op, q)
}

// LegacyToCoreCaps delegates to the package-level helper.
func (e *Enforcer) LegacyToCoreCaps(p Permission) []CoreCapability {
	return LegacyToCoreCaps(p)
}
