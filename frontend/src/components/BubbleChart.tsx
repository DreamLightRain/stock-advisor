import { useEffect, useRef, useState, useCallback } from 'react'
import { forceSimulation, forceCollide } from 'd3-force'

export interface BubbleDatum {
  name: string
  value: number
  dir: '流入' | '流出'
  changePct: number
  code: string
  ratio: number
}

interface SimNode extends BubbleDatum {
  x: number; y: number; r: number; vx: number; vy: number
}

interface Props {
  data: BubbleDatum[]
  width: number
  height: number
  fullscreen?: boolean
  onBubbleClick?: (d: BubbleDatum) => void
}

function calcRadius(abs: number, maxR: number): number {
  return Math.max(20, Math.min(maxR, 14 + Math.sqrt(abs) * 2.5))
}

export default function BubbleChart({ data, width, height, fullscreen, onBubbleClick }: Props) {
  const [nodes, setNodes] = useState<SimNode[]>([])
  const [dragging, setDragging] = useState<number | null>(null)
  const dragPos = useRef<{ x: number; y: number } | null>(null)
  const simRef = useRef<any>(null)
  const wasDragged = useRef(false)
  const bgDragging = useRef(false)
  const bgDragPos = useRef({ x: 0, y: 0 })
  const svgRef = useRef<SVGSVGElement>(null)
  const gRef = useRef<SVGGElement>(null)
  const px = useRef(0); const py = useRef(0)
  const zf = useRef(1)

  useEffect(() => {
    if (!data.length || width === 0 || height === 0) return
    const cy = height / 2
    let raw: any[]

    if (fullscreen) {
      // deterministic pyramid layout — zero overlap
      const levelRatio = [4, 3, 2, 1]
      const totalR = levelRatio.reduce((a, b) => a + b, 0)
      const levelW = [0.8, 0.6, 0.4, 0.25]

      function layoutSide(list: any[], dir: number): any[] {
        if (!list.length) return []
        const halfH = dir < 0 ? cy - 10 : height - cy - 10
        const maxR = Math.min(50, Math.sqrt(width * halfH * 0.2 / list.length))
        const sorted = [...list].sort((a: any, b: any) => Math.abs(a.value) - Math.abs(b.value))
        const levels: any[][] = [[], [], [], []]
        let idx = 0
        for (let lv = 0; lv < 4; lv++) {
          const n = Math.round((levelRatio[lv] / totalR) * sorted.length)
          for (let i = 0; i < n && idx < sorted.length; i++) levels[lv].push(sorted[idx++])
        }
        while (idx < sorted.length) levels[0].push(sorted[idx++])
        const result: any[] = []
        for (let lv = 0; lv < 4; lv++) {
          const arr = levels[lv]
          if (!arr.length) continue
          const yBase = cy + dir * (halfH * (lv + 0.5) / 4)
          const bandW = levelW[lv] * width
          const left = (width - bandW) / 2
          const spacing = bandW / (arr.length + 1)
          arr.forEach((d: any, i: number) => {
            const r = calcRadius(Math.abs(d.value), maxR)
            result.push({ ...d, r, x: left + spacing * (i + 1), y: yBase })
          })
        }
        return result
      }

      raw = [
        ...layoutSide(data.filter(d => d.dir === '流入'), -1),
        ...layoutSide(data.filter(d => d.dir === '流出'), 1),
      ]
    } else {
      // force simulation layout for compact mode
      const cx = width / 2
      const maxAbs = Math.max(...data.map(d => Math.abs(d.value)), 0.1)
      const maxR = Math.min(34, Math.sqrt(width * height * 0.18 / data.length))
      const spreadY = height * 0.36
      raw = data.map((d) => {
        const r = calcRadius(Math.abs(d.value), maxR)
        const ratio = Math.max(0.12, Math.abs(d.value) / maxAbs)
        return { ...d, r,
          x: cx + (Math.random() - 0.5) * width,
          y: d.dir === '流入' ? cy - ratio * spreadY - r : cy + ratio * spreadY + r,
        }
      })
      raw.forEach((n: any) => { n.fy = n.y })
      const sim = forceSimulation(raw)
        .force('collide', forceCollide((n: any) => n.r + 6).strength(1))
        .alphaDecay(0.05).alphaMin(0.001)
      sim.tick(300)
      for (let iter = 0; iter < 100; iter++) {
        let moved = false
        for (let i = 0; i < raw.length; i++) {
          for (let j = i + 1; j < raw.length; j++) {
            if (raw[i].dir !== raw[j].dir) continue
            let dx = raw[i].x - raw[j].x
            const dy = raw[i].y - raw[j].y
            const dist = Math.sqrt(dx * dx + dy * dy)
            const minDist = raw[i].r + raw[j].r + 4
            if (dist < minDist && dist > 0.01) {
              moved = true
              const overlap = (minDist - dist) / 2
              dx = dx / dist
              raw[i].x += dx * overlap; raw[j].x -= dx * overlap
            }
          }
        }
        for (const n of raw) {
          if (n.x - n.r < 0) n.x = n.r
          if (n.x + n.r > width) n.x = width - n.r
        }
        if (!moved) break
      }
      raw.forEach((n: any) => { n.fy = null })
      sim.stop()
    }

    setNodes([...raw])
    px.current = 0; py.current = 0; zf.current = 1
    if (gRef.current) gRef.current.setAttribute('transform', 'translate(0,0) scale(1)')
  }, [data, width, height, fullscreen])

  const handleBubbleDown = useCallback((idx: number, e: React.MouseEvent) => {
    if (!simRef.current) return
    wasDragged.current = false
    setDragging(idx)
    const sim = simRef.current
    if (!sim.nodes) return
    const node = sim.nodes()[idx]
    if (!node) return
    dragPos.current = { x: e.clientX, y: e.clientY }
    node.fx = node.x; node.fy = node.y
  }, [])

  const handleBubbleUp = useCallback((idx: number, d: SimNode) => {
    if (!wasDragged.current) onBubbleClick?.(d)
    setDragging(null)
    if (!simRef.current) return
    const sim = simRef.current
    if (!sim.nodes) return
    const node = sim.nodes()[idx]
    if (!node) return
    node.fx = null; node.fy = null
    dragPos.current = null
  }, [onBubbleClick])

  useEffect(() => {
    if (dragging === null) return
    const mm = (e: MouseEvent) => {
      if (!dragPos.current || !simRef.current) return
      const ddx = e.clientX - dragPos.current.x
      const ddy = e.clientY - dragPos.current.y
      if (Math.abs(ddx) > 2 || Math.abs(ddy) > 2) wasDragged.current = true
      dragPos.current = { x: e.clientX, y: e.clientY }
      const node = simRef.current.nodes()[dragging]
      node.fx! += ddx; node.fy! += ddy
    }
    window.addEventListener('mousemove', mm)
    window.addEventListener('mouseup', () => handleBubbleUp(dragging, simRef.current?.nodes()[dragging]), { once: true })
    return () => { window.removeEventListener('mousemove', mm) }
  }, [dragging, handleBubbleUp])

  useEffect(() => {
    if (!fullscreen) { bgDragging.current = false; return }

    const onDown = (e: MouseEvent) => {
      const tag = (e.target as HTMLElement).tagName
      if (tag === 'circle' || tag === 'text') return
      bgDragging.current = true
      bgDragPos.current = { x: e.clientX, y: e.clientY }
    }
    const onMove = (e: MouseEvent) => {
      if (!bgDragging.current || !gRef.current) return
      const dx = e.clientX - bgDragPos.current.x
      const dy = e.clientY - bgDragPos.current.y
      bgDragPos.current = { x: e.clientX, y: e.clientY }
      px.current += dx; py.current += dy
      gRef.current.setAttribute('transform', `translate(${px.current},${py.current}) scale(${zf.current})`)
    }
    const onUp = () => { bgDragging.current = false }
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const nz = Math.max(0.3, Math.min(5, zf.current - e.deltaY * 0.002))
      zf.current = nz
      if (gRef.current) gRef.current.setAttribute('transform', `translate(${px.current},${py.current}) scale(${nz})`)
    }

    const svg = svgRef.current
    if (!svg) return
    svg.addEventListener('mousedown', onDown)
    svg.addEventListener('wheel', onWheel, { passive: false })
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      svg.removeEventListener('mousedown', onDown)
      svg.removeEventListener('wheel', onWheel)
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [fullscreen])

  if (!nodes.length) return null
  const textColor = fullscreen ? '#333' : '#fff'
  const valColor = fullscreen ? '#666' : 'rgba(255,255,255,0.9)'

  return (
    <svg
      ref={svgRef}
      width={width} height={height}
      style={{ display: 'block', cursor: dragging !== null ? 'grabbing' : (fullscreen && bgDragging.current ? 'grabbing' : 'grab') }}
    >
      <g ref={gRef} transform="translate(0,0) scale(1)">
        <line x1={0} y1={height / 2} x2={width} y2={height / 2}
          stroke="#d9d9d9" strokeWidth={1} strokeDasharray="4 4" />
        {nodes.map((n, i) => {
          const isInflow = n.dir === '流入'
          const cFill = fullscreen ? (isInflow ? '#ffccc7' : '#d9f7be') : `url(#bgrad-${i})`
          const cStroke = fullscreen ? (isInflow ? '#ff4d4f' : '#73d13d') : (isInflow ? '#f5222d' : '#52c41a')
          return (
            <g key={n.code}>
              {!fullscreen && (
                <defs>
                  <radialGradient id={`bgrad-${i}`} cx="30%" cy="30%">
                    <stop offset="0%" stopColor={isInflow ? '#ff7a7a' : '#7acc7a'} stopOpacity={0.85} />
                    <stop offset="100%" stopColor={isInflow ? '#f5222d' : '#52c41a'} stopOpacity={0.6} />
                  </radialGradient>
                </defs>
              )}
              <circle
                cx={n.x} cy={n.y} r={n.r}
                fill={cFill} stroke={cStroke} strokeWidth={1.5}
                opacity={fullscreen ? 1 : 0.85}
                onMouseDown={(e) => handleBubbleDown(i, e)}
                onMouseUp={(e) => handleBubbleUp(i, n)}
              />
              <text
                x={n.x} y={n.y - (n.r >= 26 ? 4 : 1)}
                textAnchor="middle" fill={textColor}
                fontSize={Math.min(9, n.r / 3)} fontWeight={700}
                style={{ pointerEvents: 'none', userSelect: 'none' }}
              >
                {n.name}
              </text>
              <text
                x={n.x} y={n.y + (n.r >= 26 ? 11 : 8)}
                textAnchor="middle" fill={valColor}
                fontSize={Math.min(8, n.r / 3.5)} fontWeight={500}
                style={{ pointerEvents: 'none', userSelect: 'none' }}
              >
                {n.value >= 0 ? '+' : ''}{n.value.toFixed(1)}亿
              </text>
            </g>
          )
        })}
      </g>
    </svg>
  )
}
