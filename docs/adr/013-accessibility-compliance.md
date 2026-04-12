# ADR-013: Accessibility Compliance (VPAT/Section 508)

## Status: Accepted

## Context

Google Workspace and Microsoft 365 provide VPAT (Voluntary Product Accessibility Template) documentation and comply with Section 508 of the Rehabilitation Act. While accessibility is primarily a frontend concern, the database API must not create barriers.

## Decision

### Database-level accessibility support

The database itself doesn't render UI, but it must:

1. **Support text alternatives**: every node type should support a `description` or `alt_text` field for rich content (images, charts, media).

2. **Support structured content**: payloads should support structured markup (not just plain text) so frontends can render accessible content.

3. **Search accessibility**: FTS5 search must handle screen reader-friendly queries (no special syntax required).

4. **API error messages**: gRPC error messages must be human-readable strings (not just error codes) for assistive technology.

5. **Keyboard-navigable admin console**: if EntDB ships an admin UI, it must be WCAG 2.1 AA compliant.

### Schema support

```protobuf
message MediaNode {
    option (entdb.type_id) = 50;

    string url = 1;
    string alt_text = 2     [(entdb.required) = true];  // accessibility: alt text required
    string caption = 3;
    string mime_type = 4;
}
```

The `entdb lint` can warn if a media-type node doesn't have an `alt_text` field.

### Documentation

EntDB provides a VPAT document covering:
- API accessibility (error messages, structured responses)
- Admin console accessibility (if applicable)
- SDK documentation accessibility (screen reader compatible docs)

## Consequences

- Media node types should require alt_text (enforced by lint)
- API error messages must be descriptive
- Documentation site must be WCAG 2.1 AA compliant
- Admin console (if built) must be keyboard-navigable
