import { Routes, Route } from 'react-router-dom'
import Layout from './components/Layout'
import HomePage from './pages/HomePage'
import TypeListPage from './pages/TypeListPage'
import NodeListPage from './pages/NodeListPage'
import NodeDetailPage from './pages/NodeDetailPage'
import SearchPage from './pages/SearchPage'
import GraphPage from './pages/GraphPage'
import MailboxPage from './pages/MailboxPage'

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
        <Route path="/mailbox/:userId" element={<MailboxPage />} />
      </Routes>
    </Layout>
  )
}

export default App
