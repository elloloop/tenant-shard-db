import { Tabs, TabsList, TabsTrigger, TabsContent } from '@refraction-ui/react'

const sections = [
  {
    title: "Getting Started",
    items: [
      { label: "Introduction", href: "/tenant-shard-db/" },
      { label: "Installation", href: "/tenant-shard-db/docs/installation" },
      { label: "Quick Start", href: "/tenant-shard-db/docs/quickstart" },
      { label: "Core Concepts", href: "/tenant-shard-db/docs/concepts" },
    ],
  },
  {
    title: "Setup",
    items: [
      { label: "Define Schema (.proto)", href: "/tenant-shard-db/docs/setup/schema" },
      { label: "Generate Types", href: "/tenant-shard-db/docs/setup/codegen" },
      { label: "CI Integration", href: "/tenant-shard-db/docs/setup/ci" },
      { label: "Docker Compose", href: "/tenant-shard-db/docs/setup/docker" },
    ],
  },
  {
    title: "Data Operations",
    items: [
      { label: "Create & Update", href: "/tenant-shard-db/docs/data/write" },
      { label: "Query & Read", href: "/tenant-shard-db/docs/data/read" },
      { label: "Atomic Transactions", href: "/tenant-shard-db/docs/data/transactions" },
      { label: "Read-after-Write", href: "/tenant-shard-db/docs/data/consistency" },
    ],
  },
  {
    title: "Access Control",
    items: [
      { label: "Overview", href: "/tenant-shard-db/docs/acl/overview" },
      { label: "Sharing & Groups", href: "/tenant-shard-db/docs/acl/sharing" },
      { label: "Inheritance", href: "/tenant-shard-db/docs/acl/inheritance" },
      { label: "Cross-Tenant", href: "/tenant-shard-db/docs/acl/cross-tenant" },
    ],
  },
  {
    title: "GDPR & Compliance",
    items: [
      { label: "Data Classification", href: "/tenant-shard-db/docs/gdpr/classification" },
      { label: "User Deletion", href: "/tenant-shard-db/docs/gdpr/deletion" },
      { label: "Export & Portability", href: "/tenant-shard-db/docs/gdpr/export" },
      { label: "Audit Log", href: "/tenant-shard-db/docs/gdpr/audit" },
    ],
  },
  {
    title: "Common Patterns",
    items: [
      { label: "Task Management", href: "/tenant-shard-db/docs/patterns/tasks" },
      { label: "Chat / Messaging", href: "/tenant-shard-db/docs/patterns/chat" },
      { label: "Family App", href: "/tenant-shard-db/docs/patterns/family" },
      { label: "Learning Platform", href: "/tenant-shard-db/docs/patterns/learning" },
      { label: "Social Network", href: "/tenant-shard-db/docs/patterns/social" },
    ],
  },
  {
    title: "SDK Reference",
    items: [
      { label: "Python SDK", href: "/tenant-shard-db/docs/sdk/python" },
      { label: "Go SDK", href: "/tenant-shard-db/docs/sdk/go" },
      { label: "gRPC API", href: "/tenant-shard-db/docs/api/grpc" },
    ],
  },
  {
    title: "Deployment",
    items: [
      { label: "WAL Backends", href: "/tenant-shard-db/docs/deploy/wal" },
      { label: "Storage Backends", href: "/tenant-shard-db/docs/deploy/storage" },
      { label: "Multi-Node", href: "/tenant-shard-db/docs/deploy/multi-node" },
      { label: "Encryption", href: "/tenant-shard-db/docs/deploy/encryption" },
    ],
  },
]

export default function DocsSidebar({ currentPath }: { currentPath: string }) {
  return (
    <nav className="docs-sidebar">
      {sections.map((section) => (
        <div key={section.title} className="sidebar-section">
          <div className="sidebar-section-title">{section.title}</div>
          {section.items.map((item) => (
            <a
              key={item.href}
              href={item.href}
              className={`sidebar-link ${currentPath === item.href || currentPath === item.href + '/' ? 'active' : ''}`}
            >
              {item.label}
            </a>
          ))}
        </div>
      ))}
    </nav>
  )
}
