import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { ChevronLeft, ZoomIn, ZoomOut, Maximize2, Eye } from 'lucide-react'
import { api, Node } from '../api'

interface GraphNode extends Node {
  x: number
  y: number
  vx: number
  vy: number
}

export default function GraphPage() {
  const { nodeId } = useParams<{ nodeId: string }>()
  const navigate = useNavigate()
  const [depth, setDepth] = useState(1)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [zoom, setZoom] = useState(1)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [selectedNode, setSelectedNode] = useState<string | null>(null)
  const [nodes, setNodes] = useState<GraphNode[]>([])
  const [isDragging, setIsDragging] = useState(false)
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 })

  const { data: graphData, isLoading } = useQuery({
    queryKey: ['graph', nodeId, depth],
    queryFn: () => api.getGraph(nodeId!, depth),
    enabled: !!nodeId,
  })

  const { data: schema } = useQuery({
    queryKey: ['schema'],
    queryFn: () => api.getSchema(),
  })

  // Initialize node positions
  useEffect(() => {
    if (!graphData) return

    const centerX = 400
    const centerY = 300
    const radius = 150

    const initialNodes: GraphNode[] = graphData.nodes.map((node, i) => {
      const isRoot = node.id === graphData.root_id
      const angle = (2 * Math.PI * i) / graphData.nodes.length

      return {
        ...node,
        x: isRoot ? centerX : centerX + Math.cos(angle) * radius,
        y: isRoot ? centerY : centerY + Math.sin(angle) * radius,
        vx: 0,
        vy: 0,
      }
    })

    setNodes(initialNodes)
  }, [graphData])

  // Force-directed layout simulation
  useEffect(() => {
    if (nodes.length === 0 || !graphData) return

    const simulate = () => {
      setNodes(prevNodes => {
        const newNodes = [...prevNodes]

        // Apply forces
        for (let i = 0; i < newNodes.length; i++) {
          for (let j = i + 1; j < newNodes.length; j++) {
            const dx = newNodes[j].x - newNodes[i].x
            const dy = newNodes[j].y - newNodes[i].y
            const dist = Math.sqrt(dx * dx + dy * dy) || 1
            const force = 1000 / (dist * dist)

            newNodes[i].vx -= (dx / dist) * force
            newNodes[i].vy -= (dy / dist) * force
            newNodes[j].vx += (dx / dist) * force
            newNodes[j].vy += (dy / dist) * force
          }
        }

        // Apply edge springs
        for (const edge of graphData.edges) {
          const source = newNodes.find(n => n.id === edge.from_id)
          const target = newNodes.find(n => n.id === edge.to_id)
          if (!source || !target) continue

          const dx = target.x - source.x
          const dy = target.y - source.y
          const dist = Math.sqrt(dx * dx + dy * dy) || 1
          const force = (dist - 100) * 0.05

          source.vx += (dx / dist) * force
          source.vy += (dy / dist) * force
          target.vx -= (dx / dist) * force
          target.vy -= (dy / dist) * force
        }

        // Apply velocities with damping
        for (const node of newNodes) {
          node.vx *= 0.9
          node.vy *= 0.9
          node.x += node.vx
          node.y += node.vy

          // Keep nodes in bounds
          node.x = Math.max(50, Math.min(750, node.x))
          node.y = Math.max(50, Math.min(550, node.y))
        }

        return newNodes
      })
    }

    const interval = setInterval(simulate, 50)
    return () => clearInterval(interval)
  }, [nodes.length, graphData])

  // Draw graph
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || !graphData) return

    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Clear
    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // Apply transform
    ctx.save()
    ctx.translate(pan.x, pan.y)
    ctx.scale(zoom, zoom)

    // Draw edges
    ctx.strokeStyle = '#94a3b8'
    ctx.lineWidth = 1
    for (const edge of graphData.edges) {
      const source = nodes.find(n => n.id === edge.from_id)
      const target = nodes.find(n => n.id === edge.to_id)
      if (!source || !target) continue

      ctx.beginPath()
      ctx.moveTo(source.x, source.y)
      ctx.lineTo(target.x, target.y)
      ctx.stroke()

      // Arrow
      const angle = Math.atan2(target.y - source.y, target.x - source.x)
      const arrowX = target.x - Math.cos(angle) * 25
      const arrowY = target.y - Math.sin(angle) * 25

      ctx.beginPath()
      ctx.moveTo(arrowX, arrowY)
      ctx.lineTo(
        arrowX - 10 * Math.cos(angle - Math.PI / 6),
        arrowY - 10 * Math.sin(angle - Math.PI / 6)
      )
      ctx.moveTo(arrowX, arrowY)
      ctx.lineTo(
        arrowX - 10 * Math.cos(angle + Math.PI / 6),
        arrowY - 10 * Math.sin(angle + Math.PI / 6)
      )
      ctx.stroke()
    }

    // Draw nodes
    for (const node of nodes) {
      const isRoot = node.id === graphData.root_id
      const isSelected = node.id === selectedNode

      // Node circle
      ctx.beginPath()
      ctx.arc(node.x, node.y, 20, 0, 2 * Math.PI)
      ctx.fillStyle = isRoot ? '#3b82f6' : isSelected ? '#10b981' : '#6366f1'
      ctx.fill()

      if (isSelected) {
        ctx.strokeStyle = '#065f46'
        ctx.lineWidth = 3
        ctx.stroke()
      }

      // Label
      const typeName = schema?.node_types.find(t => t.type_id === node.type_id)?.name || 'Node'
      ctx.fillStyle = '#1f2937'
      ctx.font = '12px sans-serif'
      ctx.textAlign = 'center'
      ctx.fillText(typeName, node.x, node.y + 35)
      ctx.font = '10px monospace'
      ctx.fillStyle = '#6b7280'
      ctx.fillText(node.id.slice(0, 8), node.x, node.y + 48)
    }

    ctx.restore()
  }, [nodes, graphData, schema, zoom, pan, selectedNode])

  // Handle canvas click
  const handleCanvasClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    if (!canvas) return

    const rect = canvas.getBoundingClientRect()
    const x = (e.clientX - rect.left - pan.x) / zoom
    const y = (e.clientY - rect.top - pan.y) / zoom

    // Find clicked node
    for (const node of nodes) {
      const dx = x - node.x
      const dy = y - node.y
      if (dx * dx + dy * dy < 400) {
        setSelectedNode(node.id)
        return
      }
    }
    setSelectedNode(null)
  }

  // Handle canvas drag for panning
  const handleMouseDown = (e: React.MouseEvent) => {
    setIsDragging(true)
    setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y })
  }

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!isDragging) return
    setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y })
  }

  const handleMouseUp = () => {
    setIsDragging(false)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
      </div>
    )
  }

  const selectedNodeData = selectedNode ? nodes.find(n => n.id === selectedNode) : null

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="bg-white border-b border-gray-200 px-6 py-4 flex items-center justify-between">
        <div>
          <Link
            to={`/nodes/${nodeId}`}
            className="text-blue-600 hover:text-blue-800 text-sm flex items-center gap-1 mb-1"
          >
            <ChevronLeft className="w-4 h-4" />
            Back to Node
          </Link>
          <h1 className="text-xl font-bold text-gray-900">Graph View</h1>
        </div>

        <div className="flex items-center gap-4">
          {/* Depth selector */}
          <div className="flex items-center gap-2">
            <label className="text-sm text-gray-600">Depth:</label>
            <select
              value={depth}
              onChange={(e) => setDepth(Number(e.target.value))}
              className="border rounded px-2 py-1 text-sm"
            >
              <option value={1}>1</option>
              <option value={2}>2</option>
              <option value={3}>3</option>
            </select>
          </div>

          {/* Zoom controls */}
          <div className="flex items-center gap-1 border rounded">
            <button
              onClick={() => setZoom(z => Math.max(0.5, z - 0.1))}
              className="p-2 hover:bg-gray-100"
            >
              <ZoomOut className="w-4 h-4" />
            </button>
            <span className="px-2 text-sm">{Math.round(zoom * 100)}%</span>
            <button
              onClick={() => setZoom(z => Math.min(2, z + 0.1))}
              className="p-2 hover:bg-gray-100"
            >
              <ZoomIn className="w-4 h-4" />
            </button>
            <button
              onClick={() => { setZoom(1); setPan({ x: 0, y: 0 }); }}
              className="p-2 hover:bg-gray-100 border-l"
            >
              <Maximize2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Canvas and sidebar */}
      <div className="flex-1 flex">
        <div className="flex-1 bg-gray-50 relative">
          <canvas
            ref={canvasRef}
            width={800}
            height={600}
            onClick={handleCanvasClick}
            onMouseDown={handleMouseDown}
            onMouseMove={handleMouseMove}
            onMouseUp={handleMouseUp}
            onMouseLeave={handleMouseUp}
            className="w-full h-full cursor-grab active:cursor-grabbing"
          />

          <div className="absolute bottom-4 left-4 bg-white rounded-lg shadow px-4 py-2 text-sm text-gray-600">
            {graphData?.nodes.length} nodes, {graphData?.edges.length} edges
          </div>
        </div>

        {/* Selected node panel */}
        {selectedNodeData && (
          <div className="w-80 bg-white border-l border-gray-200 p-4 overflow-y-auto">
            <h3 className="font-semibold text-gray-900 mb-4">
              {schema?.node_types.find(t => t.type_id === selectedNodeData.type_id)?.name || 'Node'}
            </h3>

            <div className="space-y-4">
              <div>
                <p className="text-xs text-gray-500 uppercase">ID</p>
                <p className="text-sm font-mono text-gray-900 break-all">
                  {selectedNodeData.id}
                </p>
              </div>

              <div>
                <p className="text-xs text-gray-500 uppercase">Owner</p>
                <p className="text-sm text-gray-900">{selectedNodeData.owner_actor}</p>
              </div>

              <div>
                <p className="text-xs text-gray-500 uppercase mb-2">Payload</p>
                <pre className="text-xs bg-gray-50 p-2 rounded overflow-x-auto">
                  {JSON.stringify(selectedNodeData.payload, null, 2)}
                </pre>
              </div>

              <div className="pt-4 border-t border-gray-200 flex gap-2">
                <Link
                  to={`/nodes/${selectedNodeData.id}`}
                  className="flex-1 flex items-center justify-center gap-2 px-3 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
                >
                  <Eye className="w-4 h-4" />
                  View Details
                </Link>
                <button
                  onClick={() => navigate(`/graph/${selectedNodeData.id}`)}
                  className="px-3 py-2 border border-gray-300 rounded hover:bg-gray-50 text-sm"
                >
                  Center
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
