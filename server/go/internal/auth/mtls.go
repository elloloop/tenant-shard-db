package auth

import (
	"context"
	"crypto/x509"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// IdentityFromPeerCertificate extracts the verified mTLS client
// certificate identity from ctx, when the gRPC transport has one.
func IdentityFromPeerCertificate(ctx context.Context) (Identity, bool) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.AuthInfo == nil {
		return Identity{}, false
	}
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsInfo.State.PeerCertificates) == 0 {
		return Identity{}, false
	}
	return IdentityFromCertificate(tlsInfo.State.PeerCertificates[0])
}

// IdentityFromCertificate maps a verified client certificate to an EntDB
// trusted Identity. URI SANs are preferred, then DNS SANs, email SANs,
// CommonName, and finally the full subject DN.
func IdentityFromCertificate(cert *x509.Certificate) (Identity, bool) {
	if cert == nil {
		return Identity{}, false
	}
	subject := ""
	switch {
	case len(cert.URIs) > 0:
		subject = cert.URIs[0].String()
	case len(cert.DNSNames) > 0:
		subject = cert.DNSNames[0]
	case len(cert.EmailAddresses) > 0:
		subject = cert.EmailAddresses[0]
	case cert.Subject.CommonName != "":
		subject = cert.Subject.CommonName
	default:
		subject = cert.Subject.String()
	}
	if subject == "" {
		return Identity{}, false
	}
	return Identity{
		Method:  MethodMTLS,
		Subject: "system:" + subject,
		Metadata: map[string]any{
			"subject_dn":    cert.Subject.String(),
			"serial_number": cert.SerialNumber.String(),
			"dns_sans":      append([]string(nil), cert.DNSNames...),
			"email_sans":    append([]string(nil), cert.EmailAddresses...),
			"uri_sans":      certificateURIs(cert),
		},
	}, true
}

// MTLSPeerIdentityUnaryInterceptor installs an mTLS Identity on the
// request context when the transport presented a verified client cert.
func MTLSPeerIdentityUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if id, ok := IdentityFromPeerCertificate(ctx); ok {
			ctx = WithIdentity(ctx, id)
		}
		return handler(ctx, req)
	}
}

// MTLSPeerIdentityStreamInterceptor is the stream equivalent of
// MTLSPeerIdentityUnaryInterceptor.
func MTLSPeerIdentityStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		if id, ok := IdentityFromPeerCertificate(ctx); ok {
			ctx = WithIdentity(ctx, id)
			ss = &serverStreamWithContext{ServerStream: ss, ctx: ctx}
		}
		return handler(srv, ss)
	}
}

func certificateURIs(cert *x509.Certificate) []string {
	out := make([]string, 0, len(cert.URIs))
	for _, uri := range cert.URIs {
		out = append(out, uri.String())
	}
	return out
}
