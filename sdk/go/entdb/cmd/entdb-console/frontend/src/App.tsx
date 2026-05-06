import { Routes, Route, Navigate } from 'react-router-dom'
import Layout from './components/Layout'
import { TenantsPage } from './pages/TenantsPage'
import { TenantDetailPage } from './pages/TenantDetailPage'
import { SchemaPage } from './pages/SchemaPage'
import { NodesPage } from './pages/NodesPage'
import { NodeDetailPage } from './pages/NodeDetailPage'
import { GraphPage } from './pages/GraphPage'
import { SearchPage } from './pages/SearchPage'

export default function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/tenants" replace />} />
        <Route path="/tenants" element={<TenantsPage />} />
        <Route path="/tenants/:tenantId" element={<TenantDetailPage />} />
        <Route path="/tenants/:tenantId/schema" element={<SchemaPage />} />
        <Route path="/tenants/:tenantId/nodes" element={<NodesPage />} />
        <Route path="/tenants/:tenantId/nodes/:nodeId" element={<NodeDetailPage />} />
        <Route path="/tenants/:tenantId/nodes/:nodeId/graph" element={<GraphPage />} />
        <Route path="/tenants/:tenantId/search" element={<SearchPage />} />
        <Route path="*" element={<Navigate to="/tenants" replace />} />
      </Routes>
    </Layout>
  )
}
