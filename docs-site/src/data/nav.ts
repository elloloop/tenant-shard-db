// Single source of truth for the documentation nav. Used by:
//   - the sidebar (rendered in DocsLayout)
//   - breadcrumbs
//   - prev/next links at the bottom of every page
//
// Keep `href` paths in sync with the file paths under src/pages/.

export interface NavItem {
  label: string;
  href: string;
}

export interface NavSection {
  title: string;
  items: NavItem[];
}

export const BASE = "/tenant-shard-db";

export const sidebarSections: NavSection[] = [
  {
    title: "Getting Started",
    items: [
      { label: "Introduction", href: `${BASE}/` },
      { label: "Installation", href: `${BASE}/docs/installation` },
      { label: "Quick Start", href: `${BASE}/docs/quickstart` },
      { label: "Core Concepts", href: `${BASE}/docs/concepts` },
    ],
  },
  {
    title: "Setup",
    items: [
      { label: "Define Schema (.proto)", href: `${BASE}/docs/setup/schema` },
      { label: "CI Integration", href: `${BASE}/docs/setup/ci` },
      { label: "Schema CLI", href: `${BASE}/docs/schema/cli` },
      { label: "Schema Evolution", href: `${BASE}/docs/schema/evolution` },
    ],
  },
  {
    title: "Data & ACL",
    items: [
      { label: "Read-after-Write", href: `${BASE}/docs/crud/consistency` },
      { label: "ACL Overview", href: `${BASE}/docs/acl/overview` },
    ],
  },
  {
    title: "Common Patterns",
    items: [
      { label: "Task Management", href: `${BASE}/docs/patterns/tasks` },
      { label: "Family App", href: `${BASE}/docs/patterns/family` },
      { label: "Learning Platform", href: `${BASE}/docs/patterns/learning` },
    ],
  },
  {
    title: "SDK Reference",
    items: [
      { label: "Python SDK", href: `${BASE}/docs/sdk/python` },
      { label: "Go SDK", href: `${BASE}/docs/sdk/go` },
    ],
  },
  {
    title: "Compliance",
    items: [
      { label: "Audit & Object Lock", href: `${BASE}/docs/compliance/audit` },
      { label: "GDPR", href: `${BASE}/docs/compliance/gdpr` },
    ],
  },
  {
    title: "Deployment",
    items: [{ label: "WAL Backends", href: `${BASE}/docs/deploy/wal` }],
  },
];

// Flat ordered list with section labels — drives breadcrumbs and prev/next.
export interface FlatNavItem extends NavItem {
  section: string;
}

export const flatNav: FlatNavItem[] = sidebarSections.flatMap((section) =>
  section.items.map((item) => ({ ...item, section: section.title })),
);

// Normalize a path to match how `href` is defined above (strip trailing slash,
// keep root as just the BASE).
function normalize(p: string): string {
  if (!p) return p;
  if (p === BASE || p === `${BASE}/`) return `${BASE}/`;
  return p.replace(/\/+$/, "");
}

export function findCurrent(currentPath: string): FlatNavItem | undefined {
  const target = normalize(currentPath);
  return flatNav.find(
    (item) => normalize(item.href) === target || item.href === target,
  );
}

export function findPrevNext(currentPath: string): {
  prev?: FlatNavItem;
  next?: FlatNavItem;
} {
  const target = normalize(currentPath);
  const idx = flatNav.findIndex(
    (item) => normalize(item.href) === target || item.href === target,
  );
  if (idx === -1) return {};
  return {
    prev: idx > 0 ? flatNav[idx - 1] : undefined,
    next: idx < flatNav.length - 1 ? flatNav[idx + 1] : undefined,
  };
}

export function buildBreadcrumbs(
  currentPath: string,
): { label: string; href?: string }[] {
  const current = findCurrent(currentPath);
  const docsRoot = { label: "Docs", href: `${BASE}/` };
  if (!current) return [docsRoot];
  // Root introduction page — just "Docs".
  if (current.href === `${BASE}/`) return [docsRoot];
  return [
    docsRoot,
    { label: current.section },
    { label: current.label },
  ];
}
