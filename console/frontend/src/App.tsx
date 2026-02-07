import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import HomePage from './pages/HomePage'
import TypeListPage from './pages/TypeListPage'
import NodeListPage from './pages/NodeListPage'
import NodeDetailPage from './pages/NodeDetailPage'
import SearchPage from './pages/SearchPage'
import GraphPage from './pages/GraphPage'

function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/types" element={<TypeListPage />} />
        <Route path="/types/:typeId" element={<NodeListPage />} />
        <Route path="/nodes/:nodeId" element={<NodeDetailPage />} />
        <Route path="/search" element={<SearchPage />} />
        <Route path="/graph/:nodeId" element={<GraphPage />} />
      </Routes>
    </Layout>
  )
}

export default App
