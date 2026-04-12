import {
  ThemeProvider,
  ThemeToggle,
  AppShell,
} from "@refraction-ui/react";
import type { ReactNode } from "react";

const navLinks = [
  { label: "Docs", href: "/tenant-shard-db/" },
  { label: "GitHub", href: "https://github.com/elloloop/tenant-shard-db" },
  { label: "PyPI", href: "https://pypi.org/project/entdb-sdk/" },
];

const sidebarSections = [
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
      { label: "CI Integration", href: "/tenant-shard-db/docs/setup/ci" },
      { label: "Schema CLI", href: "/tenant-shard-db/docs/schema/cli" },
      { label: "Schema Evolution", href: "/tenant-shard-db/docs/schema/evolution" },
    ],
  },
  {
    title: "Data & ACL",
    items: [
      { label: "Read-after-Write", href: "/tenant-shard-db/docs/crud/consistency" },
      { label: "ACL Overview", href: "/tenant-shard-db/docs/acl/overview" },
    ],
  },
  {
    title: "Common Patterns",
    items: [
      { label: "Task Management", href: "/tenant-shard-db/docs/patterns/tasks" },
      { label: "Family App", href: "/tenant-shard-db/docs/patterns/family" },
      { label: "Learning Platform", href: "/tenant-shard-db/docs/patterns/learning" },
    ],
  },
  {
    title: "SDK Reference",
    items: [{ label: "Python SDK", href: "/tenant-shard-db/docs/sdk/python" }],
  },
  {
    title: "Deployment",
    items: [{ label: "WAL Backends", href: "/tenant-shard-db/docs/deploy/wal" }],
  },
];

interface DocsShellProps {
  currentPath: string;
  children: ReactNode;
}

export default function DocsShell({ currentPath, children }: DocsShellProps) {
  return (
    <ThemeProvider>
      <AppShell config={{ mobileBreakpoint: 768, tabletBreakpoint: 1024 }}>
        <AppShell.Header>
          <div className="flex h-14 items-center gap-6 border-b border-border bg-background px-4 md:px-6">
            <AppShell.MobileNav />
            <a href="/tenant-shard-db/" className="flex items-center gap-2 font-bold">
              <div className="h-6 w-6 rounded-md bg-gradient-to-br from-primary to-primary/60" />
              <span className="text-primary">EntDB</span>
              <span className="text-foreground">Docs</span>
            </a>
            <nav className="hidden items-center gap-5 md:flex">
              {navLinks.map((link) => (
                <a
                  key={link.href}
                  href={link.href}
                  className="text-sm font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  {link.label}
                </a>
              ))}
            </nav>
            <div className="ml-auto flex items-center gap-3">
              <ThemeToggle variant="segmented" />
            </div>
          </div>
        </AppShell.Header>

        <AppShell.Sidebar>
          <nav className="flex flex-col gap-1 overflow-y-auto p-4">
            {sidebarSections.map((section) => (
              <div key={section.title} className="mb-4">
                <div className="mb-2 px-3 text-xs font-bold uppercase tracking-wider text-muted-foreground">
                  {section.title}
                </div>
                {section.items.map((item) => {
                  const isActive =
                    currentPath === item.href ||
                    currentPath === item.href + "/";
                  return (
                    <a
                      key={item.href}
                      href={item.href}
                      className={`block rounded-md px-3 py-1.5 text-sm transition-colors ${
                        isActive
                          ? "bg-primary/10 font-medium text-primary"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground"
                      }`}
                    >
                      {item.label}
                    </a>
                  );
                })}
              </div>
            ))}
          </nav>
        </AppShell.Sidebar>

        <AppShell.Main>
          <AppShell.Content>
            <article className="prose mx-auto max-w-3xl px-4 py-10 md:px-8">
              {children}
            </article>
          </AppShell.Content>
        </AppShell.Main>

        <AppShell.Overlay />
      </AppShell>
    </ThemeProvider>
  );
}
