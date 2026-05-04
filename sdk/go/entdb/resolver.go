package entdb

import (
	"fmt"
	"strings"
)

// NodeResolver maps a server-issued “node_id“ (e.g. "node-a") to
// a dial-able gRPC endpoint (“host:port“). The SDK calls this
// when the server returns a redirect hint via the
// “entdb-redirect-node“ trailing metadata header.
//
// Two implementations ship with the SDK:
//
//   - [DNSTemplateResolver] (default) treats “node_id“ as a DNS
//     subdomain — “node-a“ resolves to
//     “node-a.<BaseDomain>:<Port>“. This is the right choice for
//     Kubernetes StatefulSet headless services and for any deployment
//     that follows the convention "one DNS name per node id".
//   - [StaticMapResolver] looks up the endpoint in an explicit
//     map. Use it for tests, or for static deployments where DNS
//     isn't available.
//
// Custom implementations can plug in via [WithNodeResolver] — for
// example a service-discovery client (Consul, etcd, AWS Cloud Map).
type NodeResolver interface {
	// Resolve returns the dial-target endpoint for ``nodeID``,
	// or an error if the node is unknown.
	Resolve(nodeID string) (endpoint string, err error)
}

// DNSTemplateResolver is the default [NodeResolver]. It composes
// the endpoint as “<nodeID>.<BaseDomain>:<Port>“. “Port“
// defaults to 50051 (the EntDB gRPC port) when zero.
type DNSTemplateResolver struct {
	// BaseDomain is the parent DNS zone — typically the headless
	// service domain in Kubernetes (e.g.
	// ``entdb.svc.cluster.local``).
	BaseDomain string
	// Port is the gRPC port on each node. Zero means 50051.
	Port int
}

// Resolve composes “<nodeID>.<BaseDomain>:<Port>“.
func (r *DNSTemplateResolver) Resolve(nodeID string) (string, error) {
	if nodeID == "" {
		return "", fmt.Errorf("entdb: resolve: empty node id")
	}
	if strings.TrimSpace(r.BaseDomain) == "" {
		return "", fmt.Errorf("entdb: resolve: BaseDomain not set on DNSTemplateResolver")
	}
	port := r.Port
	if port == 0 {
		port = 50051
	}
	return fmt.Sprintf("%s.%s:%d", nodeID, r.BaseDomain, port), nil
}

// StaticMapResolver is the explicit “node_id -> endpoint“ form.
// It is the resolver of choice for tests (DNS isn't available in
// most test environments) and for tiny static deployments where
// listing endpoints by hand is fine.
type StaticMapResolver struct {
	Endpoints map[string]string
}

// Resolve looks up “nodeID“ in the static map. Returns an error
// if the node id isn't present — callers should treat that as a
// permanent failure (the SDK will surface it as a ConnectionError).
func (r *StaticMapResolver) Resolve(nodeID string) (string, error) {
	if nodeID == "" {
		return "", fmt.Errorf("entdb: resolve: empty node id")
	}
	ep, ok := r.Endpoints[nodeID]
	if !ok {
		return "", fmt.Errorf("entdb: resolve: no endpoint for node %q", nodeID)
	}
	return ep, nil
}
