import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import {
  Card, Row, Col, Statistic, Typography, Space, Input, Button, Tag, Empty, Spin, message, DatePicker, Slider, Tooltip, Popover, List, Divider, Radio, Switch
} from 'antd'
import {
  BarChartOutlined, ArrowUpOutlined, ArrowDownOutlined, SearchOutlined, RobotOutlined, ReloadOutlined, MoneyCollectOutlined, CalendarOutlined, AppstoreOutlined, ClockCircleOutlined, HistoryOutlined, ThunderboltOutlined, FundOutlined, StarOutlined, StarFilled, FullscreenOutlined, FullscreenExitOutlined, DotChartOutlined, LineChartOutlined
} from '@ant-design/icons'
import { Column, Line } from '@ant-design/charts'
import BubbleChart from '../components/BubbleChart'
import dayjs, { Dayjs } from 'dayjs'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { GetMarketSummary, GetSectorMoneyFlow, GetSectorMoneyFlowByDate, GetSectorTree, GetMoneyFlow, SearchStock, GetRealtimeData, AIChat, GetRefreshInterval, GetSelfSelectStocks, GetGroups } from '../api/bridge'
import ModelIndicator from '../components/ModelIndicator'

const { Title, Text } = Typography

function lastTradingDay(): Dayjs {
  const now = dayjs()
  if (now.day() === 0) return now.subtract(2, 'day')
  if (now.day() === 6) return now.subtract(1, 'day')
  return now
}

function isToday(date: Dayjs): boolean {
  return date.isSame(dayjs(), 'day')
}

export default function MarketAnalysis() {
  const [summary, setSummary] = useState<any>(null)
  const [indices, setIndices] = useState<any[]>([])
  const [sectors, setSectors] = useState<any[]>([])
  const [sectorTree, setSectorTree] = useState<any[]>([])
  const [sectorDate, setSectorDate] = useState<Dayjs>(dayjs())
  const [sectorLoading, setSectorLoading] = useState(false)
  const [searchedStock, setSearchedStock] = useState<any>(null)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState<any[]>([])
  const [searchPopoverOpen, setSearchPopoverOpen] = useState(false)
  const [stockMoneyFlow, setStockMoneyFlow] = useState<any[]>([])
  const [stockRealtime, setStockRealtime] = useState<any>(null)
  const [loading, setLoading] = useState(false)
  const [aiLoading, setAiLoading] = useState(false)
  const [aiResult, setAiResult] = useState('')
  const [webSearch, setWebSearch] = useState(false)
  const [refreshInterval, setRefreshInterval] = useState(2)
  const [lastRefresh, setLastRefresh] = useState(Date.now())
  const [selectedDate, setSelectedDate] = useState<Dayjs>(lastTradingDay())
  const [mfDays, setMfDays] = useState(10)
  const [selfStocks, setSelfStocks] = useState<any[]>([])
  const [selfStockRealtime, setSelfStockRealtime] = useState<Record<string, any>>({})
  const [aiAnalysisScope, setAiAnalysisScope] = useState<'market' | 'self'>('market')
  const [chartType, setChartType] = useState<'bubble' | 'line'>('bubble')
  const [timeDim, setTimeDim] = useState<'intraday' | 'daily'>('daily')
  const [chartFullscreen, setChartFullscreen] = useState(false)
  const [sortBy, setSortBy] = useState<'mainNet' | 'changePct'>('mainNet')
  const [sortDir, setSortDir] = useState<'desc' | 'asc'>('desc')
  const [markedSectors, setMarkedSectors] = useState<Set<string>>(new Set())
  const [showMarkedOnly, setShowMarkedOnly] = useState(false)
  const [topN, setTopN] = useState(20)
  const timerRef = useRef<any>(null)
  const searchTimer = useRef<any>(null)
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const [chartSize, setChartSize] = useState({ w: 600, h: 400 })

  const marketOpen = (() => {
    const now = new Date()
    const dow = now.getDay()
    if (dow === 0 || dow === 6) return false
    const h = now.getHours(), m = now.getMinutes(), t = h * 100 + m
    return (t >= 930 && t <= 1130) || (t >= 1300 && t <= 1500)
  })()

  const isTodaySelected = isToday(sectorDate)

  const loadSelfSelectData = useCallback(async () => {
    const stocks = await GetSelfSelectStocks()
    setSelfStocks(stocks || [])
    if (stocks && stocks.length > 0) {
      const codes = stocks.map((s: any) => s.info?.code || s.code).filter(Boolean)
      if (codes.length > 0) {
        const rt = await GetRealtimeData(codes)
        setSelfStockRealtime(rt || {})
      }
    }
  }, [])

  const loadMarketData = useCallback(async (date?: Dayjs) => {
    setLoading(true)
    const [sum, iv] = await Promise.all([
      GetMarketSummary(),
      GetRefreshInterval(),
    ])
    if (sum) { setSummary(sum); setIndices(sum.indices || []) }
    setRefreshInterval(iv ?? 2)
    setLastRefresh(Date.now())
    setLoading(false)
    if (!sum) {
      message.info('市场数据暂不可用（非交易时段或数据源异常）')
    }
    if (sum && sum.date) {
      const dataDate = dayjs(sum.date)
      if (date && !date.isSame(dataDate, 'day') && dataDate.isValid()) {
        message.info(`所选日期(${date.format('YYYY-MM-DD')})暂无独立数据，当前显示${dataDate.format('YYYY-MM-DD')}数据`)
      }
    }
  }, [])

  const loadSectorData = useCallback(async (d: Dayjs) => {
    setSectorLoading(true)
    const data = isToday(d)
      ? await GetSectorMoneyFlow()
      : await GetSectorMoneyFlowByDate(d.format('YYYY-MM-DD'))
    setSectors(data || [])
    const tree = isToday(d) ? await GetSectorTree() : null
    setSectorTree(tree || [])
    setLastRefresh(Date.now())
    setSectorLoading(false)
  }, [])

  useEffect(() => { loadMarketData(selectedDate) }, [loadMarketData])
  useEffect(() => { loadSectorData(sectorDate) }, [loadSectorData, sectorDate])

  const bubbleRefreshMs = useMemo(() => chartType === 'bubble' ? 30000 : refreshInterval * 1000, [chartType, refreshInterval])

  useEffect(() => {
    if (bubbleRefreshMs > 0 && marketOpen && isToday(sectorDate)) {
      timerRef.current = setInterval(() => loadSectorData(sectorDate), Math.max(bubbleRefreshMs, 10))
      return () => clearInterval(timerRef.current)
    }
  }, [bubbleRefreshMs, loadSectorData, marketOpen, sectorDate])

  const handleSectorDateChange = (d: Dayjs | null) => {
    if (!d) return
    setSectorDate(d)
    loadSectorData(d)
  }

  useEffect(() => {
    try {
      const saved = localStorage.getItem('markedSectors')
      if (saved) setMarkedSectors(new Set(JSON.parse(saved)))
    } catch {}
  }, [])

  useEffect(() => {
    const el = chartContainerRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      for (const e of entries) {
        const { width, height } = e.contentRect
        setChartSize({ w: Math.floor(width), h: Math.floor(height) })
      }
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const toggleMarkSector = (code: string) => {
    setMarkedSectors(prev => {
      const next = new Set(prev)
      if (next.has(code)) next.delete(code); else next.add(code)
      localStorage.setItem('markedSectors', JSON.stringify([...next]))
      return next
    })
  }

  const { sortedSectors, displaySectors } = useMemo(() => {
    if (!sectors.length) return { sortedSectors: [], displaySectors: [] }
    const sorted = [...sectors].sort((a: any, b: any) => {
      if (sortBy === 'changePct') return b.changePct - a.changePct
      return sortDir === 'desc' ? b.mainNet - a.mainNet : a.mainNet - b.mainNet
    })
    const display = showMarkedOnly
      ? sorted.filter((s: any) => markedSectors.has(s.code))
      : sorted
    return { sortedSectors: sorted, displaySectors: display }
  }, [sectors, sortBy, showMarkedOnly, markedSectors])

  const bubbleChartData = useMemo(() =>
    displaySectors.slice(0, topN).map((s: any) => ({
      name: s.name,
      value: s.mainNet / 1e8,
      dir: s.mainNet >= 0 ? '流入' as const : '流出' as const,
      changePct: s.changePct,
      code: s.code,
      ratio: s.mainRatio,
    })),
    [displaySectors, topN, sortedSectors]
  )

  const lineChartData = useMemo(() => {
    if (!displaySectors.length) return []
    const result: any[] = []
    const sectorsToShow = showMarkedOnly
      ? displaySectors.filter((s: any) => markedSectors.has(s.code))
      : displaySectors.slice(0, topN)
    sectorsToShow.forEach((s: any) => {
      result.push({
        sector: s.name,
        time: dayjs().format('MM-DD'),
        value: s.mainNet / 1e8,
        dir: s.mainNet >= 0 ? '流入' : '流出',
      })
    })
    return result
  }, [displaySectors, topN, showMarkedOnly, markedSectors])

  const handleSearchInput = (val: string) => {
    setSearchKeyword(val)
    if (searchTimer.current) clearTimeout(searchTimer.current)
    if (!val.trim()) { setSearchResults([]); setSearchPopoverOpen(false); return }
    searchTimer.current = setTimeout(async () => {
      const results = await SearchStock(val.trim())
      if (results && results.length > 0) {
        setSearchResults(results.slice(0, 10))
        setSearchPopoverOpen(true)
      } else {
        setSearchResults([])
        setSearchPopoverOpen(false)
      }
    }, 300)
  }

  const handleSelectSearchResult = async (item: any) => {
    setSearchedStock(item)
    setSearchPopoverOpen(false)
    setSearchKeyword(`${item.name} (${item.code})`)
    const [mf, rt] = await Promise.all([
      GetMoneyFlow(item.code, mfDays),
      GetRealtimeData([item.code]),
    ])
    setStockMoneyFlow(mf || [])
    setStockRealtime(rt ? rt[item.code] : null)
  }

  const handleSearchKeyPress = async (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && searchResults.length > 0) {
      handleSelectSearchResult(searchResults[0])
    }
  }

  const handleSelectLeaderStock = async (leader: any) => {
    const item = { code: leader.code, name: leader.name }
    setSearchedStock(item)
    setSearchKeyword(`${leader.name} (${leader.code})`)
    const [mf, rt] = await Promise.all([
      GetMoneyFlow(leader.code, mfDays),
      GetRealtimeData([leader.code]),
    ])
    setStockMoneyFlow(mf || [])
    setStockRealtime(rt ? rt[leader.code] : null)
  }

  const handleGlobalRefresh = async () => {
    await Promise.all([
      loadMarketData(selectedDate),
      loadSectorData(sectorDate),
      loadSelfSelectData(),
    ])
  }

  const handleAIAnalysis = async () => {
    setAiLoading(true)
    setAiResult('')
    try {
      let prompt = webSearch ? '[联网搜索] ' : ''

      if (aiAnalysisScope === 'self') {
        prompt += '请分析我的自选股整体情况。'
        const codes = Object.keys(selfStockRealtime)
        prompt += '\n\n当前自选股及实时行情:'
        selfStocks.forEach((s: any) => {
          const code = s.info?.code || s.code
          const rt = selfStockRealtime[code]
          if (rt) {
            prompt += `\n- ${s.info?.name || s.name}(${code}): ${rt.price} (${rt.changePercent >= 0 ? '+' : ''}${rt.changePercent?.toFixed(2)}%)`
          }
        })
        prompt += '\n\n请分析:\n1. 哪些个股有短线机会\n2. 结合板块资金流，是否有板块性机会\n3. 风险提示\n4. 操作建议'
        if (indices.length > 0) {
          prompt += '\n\n参考指数:'
          indices.forEach((idx: any) => {
            prompt += `\n${idx.name}: ${idx.price} (${idx.changePercent >= 0 ? '+' : ''}${idx.changePercent?.toFixed(2)}%)`
          })
        }
        if (sectors.length > 0) {
          prompt += '\n\n板块资金流向TOP10:'
          sectors.slice(0, 10).forEach((s: any) => {
            prompt += `\n${s.name}: ${(s.mainNet / 1e8).toFixed(2)}亿 (${s.changePct >= 0 ? '+' : ''}${s.changePct?.toFixed(2)}%)`
          })
        }
      } else if (searchedStock && stockRealtime) {
        prompt += `请分析个股 ${searchedStock.name}(${searchedStock.code})。当前价:${stockRealtime.price}, 涨跌幅:${stockRealtime.changePercent?.toFixed(2)}%`
        if (stockMoneyFlow.length > 0) {
          const last = stockMoneyFlow[stockMoneyFlow.length - 1]
          prompt += `, 主力净流入:${(last.mainNet / 1e8).toFixed(2)}亿`
        }
        prompt += '\n\n请结合市场整体情况分析该股走势，注意是否存在诱多/诱空陷阱。'
      } else {
        prompt += '请分析当前A股市场情况。'
        indices.forEach((idx: any) => {
          prompt += `\n${idx.name}: ${idx.price}点 (${idx.changePercent >= 0 ? '+' : ''}${idx.changePercent?.toFixed(2)}%), 成交额${(idx.amount / 1e8).toFixed(0)}亿`
        })
        prompt += '\n\n资金流向板块TOP5:'
        sectors.slice(0, 5).forEach((s: any) => {
          prompt += `\n${s.name}: 主力净流入${(s.mainNet / 1e8).toFixed(2)}亿`
        })
        prompt += '\n\n请分析：\n1. 市场整体资金流向和趋势\n2. 哪些板块资金明显流入/流出\n3. 是否存在诱多/诱空陷阱\n4. 操作建议'
      }
      const reply = await AIChat(prompt)
      setAiResult(reply || '无返回')
    } catch (e: any) {
      setAiResult(`AI分析失败: ${e.message}`)
    }
    setAiLoading(false)
  }

  const refreshInfo = (() => {
    if (!marketOpen) return { text: '闭市', tag: 'warning' as const }
    const elapsed = (Date.now() - lastRefresh) / 1000
    if (elapsed < 2) return { text: '最新', tag: 'green' as const }
    return { text: dayjs(lastRefresh).format('HH:mm:ss'), tag: 'default' as const }
  })()

  const lastRefreshText = dayjs(lastRefresh).format('YYYY-MM-DD HH:mm:ss')

  const renderSectorChart = () => {
    if (!displaySectors.length) {
      return <Empty description={isTodaySelected ? '暂无板块资金流向数据' : `暂未获取到 ${sectorDate.format('YYYY-MM-DD')} 的板块数据`} />
    }
    if (chartType === 'bubble') {
      const chartH = chartFullscreen ? Math.max(window.innerHeight - 100, 600) : 350
      return (
          <BubbleChart
            data={bubbleChartData}
            width={chartFullscreen ? window.innerWidth - 80 : chartSize.w}
            height={chartH}
            fullscreen={chartFullscreen}
            onBubbleClick={(d) => {
              const s = sectors.find((x: any) => x.code === d.code)
              if (s) toggleMarkSector(s.code)
            }}
          />
      )
    }
    if (timeDim === 'intraday' && !marketOpen) {
      return <Empty description="分时数据仅交易时段可用" />
    }
    return (
      <Line
        data={lineChartData}
        xField="time"
        yField="value"
        seriesField="sector"
        height={300}
        autoFit
                colorField="dir"
                scale={{ color: { domain: ['流入', '流出'], range: ['#f5222d', '#52c41a'] } }}
        point={{ shape: 'circle', size: 3 }}
        axis={{
          x: { title: null, label: { style: { fontSize: 10 } } },
          y: { title: '主力净流入(亿)' },
        }}
      />
    )
  }

  return (
    <div>
      <Row justify="space-between" align="middle" style={{ marginBottom: 16 }}>
        <Col>
          <Title level={4} style={{ margin: 0 }}>
            <BarChartOutlined /> 市场分析
          </Title>
        </Col>
        <Col>
          <Space size="small" wrap>
            {isTodaySelected ? (
              <Tag icon={<ThunderboltOutlined />} color="blue" style={{ fontSize: 11, margin: 0 }}>实时数据</Tag>
            ) : (
              <Tag icon={<HistoryOutlined />} color="purple" style={{ fontSize: 11, margin: 0 }}>{sectorDate.format('MM-DD')} 收盘数据</Tag>
            )}
            <DatePicker
              size="small"
              value={sectorDate}
              onChange={handleSectorDateChange}
              allowClear={false}
              disabledDate={(d) => d.isAfter(dayjs())}
              suffixIcon={<CalendarOutlined />}
              style={{ width: 120 }}
            />
            <Tag color={refreshInfo.tag} style={{ margin: 0, fontSize: 11 }}>
              <Tooltip title={`最新刷新: ${lastRefreshText}`}>{refreshInfo.text}</Tooltip>
            </Tag>
            <ModelIndicator compact webSearch={webSearch} onWebSearchChange={setWebSearch} />
            <Button size="small" icon={<ReloadOutlined />} onClick={handleGlobalRefresh} loading={loading}>刷新</Button>
          </Space>
        </Col>
      </Row>

      <Row gutter={12} style={{ marginBottom: 16 }}>
        {indices.length > 0 ? indices.map((idx: any) => (
          <Col span={8} key={idx.code}>
            <Card size="small" hoverable>
              <Statistic
                title={idx.name}
                value={idx.price}
                precision={2}
                prefix={idx.changePercent >= 0 ? <ArrowUpOutlined style={{ color: '#f5222d' }} /> : <ArrowDownOutlined style={{ color: '#52c41a' }} />}
                suffix={
                  <Text style={{ color: idx.changePercent >= 0 ? '#f5222d' : '#52c41a', fontSize: 14 }}>
                    {idx.changePercent >= 0 ? '+' : ''}{idx.changePercent?.toFixed(2)}%
                  </Text>
                }
              />
              <div style={{ marginTop: 8, fontSize: 12, color: '#999' }}>
                成交额: {(idx.amount / 1e8).toFixed(0)}亿
                {idx.high > 0 ? ` | 高:${idx.high} 低:${idx.low}` : ''}
              </div>
            </Card>
          </Col>
        )) : (
          <Col span={24}>
            <Card size="small"><Text type="secondary">暂无可用的指数数据</Text></Card>
          </Col>
        )}
      </Row>
      {summary?.date && (
        <div style={{ textAlign: 'right', marginBottom: 8 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>数据日期: {summary.date}</Text>
        </div>
      )}

      {/* Main content: left half sector flow, right half AI */}
      <Spin spinning={loading || sectorLoading}>
        <Row gutter={16}>
          {/* Left: Sector Funds Flow + Stock Search */}
          <Col xs={24} lg={12}>
            {/* Fuzzy Search Bar */}
            <Card size="small" style={{ marginBottom: 12 }}>
              <Row gutter={8} align="middle">
                <Col flex="auto">
                  <Popover
                    open={searchPopoverOpen}
                    onOpenChange={setSearchPopoverOpen}
                    trigger="manual"
                    placement="bottom"
                    content={
                      <List
                        size="small"
                        dataSource={searchResults}
                        style={{ width: 320, maxHeight: 300, overflow: 'auto' }}
                        renderItem={(item: any) => (
                          <List.Item
                            style={{ cursor: 'pointer', padding: '4px 8px' }}
                            onClick={() => handleSelectSearchResult(item)}
                          >
                            <Space>
                              <Tag color="blue">{item.code}</Tag>
                              <Text strong>{item.name}</Text>
                              {item.industry && <Text type="secondary" style={{ fontSize: 11 }}>{item.industry}</Text>}
                            </Space>
                          </List.Item>
                        )}
                      />
                    }
                  >
                    <Input
                      placeholder="搜索股票"
                      prefix={<SearchOutlined />}
                      value={searchKeyword}
                      onChange={(e) => handleSearchInput(e.target.value)}
                      onKeyDown={handleSearchKeyPress}
                      allowClear
                      suffix={searchedStock ? (
                        <Tag closable onClose={() => { setSearchedStock(null); setStockMoneyFlow([]); setStockRealtime(null); setSearchKeyword('') }}>
                          {searchedStock.name}
                        </Tag>
                      ) : null}
                    />
                  </Popover>
                </Col>
              </Row>
            </Card>

            {/* Sector Money Flow */}
            <Card
              title={<Space><MoneyCollectOutlined /> 板块资金流向 <Text type="secondary" style={{ fontSize: 11 }}>{sectorTree.length > 0 ? `${sectorTree.length}个一级` : `${sectors.length}个板块`}</Text></Space>}
              size="small" style={{ marginBottom: 12 }}
              extra={
                <Space size={4} wrap>
                  <Radio.Group size="small" value={timeDim} onChange={e => setTimeDim(e.target.value)}>
                    <Radio.Button value="intraday">分时</Radio.Button>
                    <Radio.Button value="daily">日</Radio.Button>
                  </Radio.Group>
                  <Divider type="vertical" />
                  <Radio.Group size="small" value={chartType} onChange={e => setChartType(e.target.value)}>
                    <Radio.Button value="bubble"><DotChartOutlined /> 气泡</Radio.Button>
                    <Radio.Button value="line"><LineChartOutlined /> 折线</Radio.Button>
                  </Radio.Group>
                  <Divider type="vertical" />
                  <Tooltip title="仅显示标记板块">
                    <Switch size="small" checked={showMarkedOnly} onChange={setShowMarkedOnly}
                      checkedChildren={<StarFilled />} unCheckedChildren={<StarOutlined />} />
                  </Tooltip>
                  <Button size="small" icon={chartFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
                    onClick={() => setChartFullscreen(!chartFullscreen)} />
                </Space>
              }
            >
              <div ref={chartContainerRef} style={chartFullscreen ? { position: 'fixed', top: 0, left: 0, width: '100vw', height: '100vh', zIndex: 9999, background: '#fff', padding: 40, overflow: 'auto' } : {}}>
                {chartFullscreen && (
                  <Button type="primary" icon={<FullscreenExitOutlined />}
                    onClick={() => setChartFullscreen(false)}
                    style={{ position: 'fixed', top: 16, right: 16, zIndex: 10000 }}>
                    退出全屏
                  </Button>
                )}
                {chartType === 'line' && timeDim === 'daily' && displaySectors.length > 0 && (
                  <div style={{ marginBottom: 8 }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>显示前</Text>
                    <Slider min={5} max={50} step={5} value={topN}
                      onChange={setTopN}
                      style={{ width: 120, display: 'inline-block', margin: '0 8px' }}
                      tooltip={{ formatter: (v) => `${v}个` }} />
                    <Text type="secondary" style={{ fontSize: 11 }}>个板块</Text>
                  </div>
                )}
                {!chartFullscreen && chartType === 'bubble' && (
                  <div style={{ marginBottom: 8 }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>显示前</Text>
                    <Slider min={10} max={100} step={10} value={topN}
                      onChange={setTopN}
                      style={{ width: 100, display: 'inline-block', margin: '0 8px' }}
                      tooltip={{ formatter: (v) => `${v}个` }} />
                    <Text type="secondary" style={{ fontSize: 11 }}>个板块</Text>
                    <Divider type="vertical" />
                    <Text type="secondary" style={{ fontSize: 11 }}>排序:</Text>
                    <Radio.Group size="small" value={sortBy} onChange={e => {
                      const val = e.target.value
                      if (val === 'mainNet') {
                        setSortDir(prev => prev === 'desc' ? 'asc' : 'desc')
                      }
                      setSortBy(val)
                    }} style={{ marginLeft: 4 }}>
                      <Radio.Button value="mainNet">净额{sortBy === 'mainNet' ? (sortDir === 'desc' ? ' ↓' : ' ↑') : ''}</Radio.Button>
                      <Radio.Button value="changePct">涨跌</Radio.Button>
                    </Radio.Group>
                  </div>
                )}
                {renderSectorChart()}
              </div>
              {!chartFullscreen && (
                <div style={{ maxHeight: 300, overflowY: 'auto', marginTop: 8 }}>
                  {sectorTree.length > 0 ? (
                    <table style={{ width: '100%', fontSize: 11, borderCollapse: 'collapse' }}>
                      <thead>
                        <tr style={{ background: '#fafafa', position: 'sticky', top: 0 }}>
                          <th style={{ padding: '2px 4px', textAlign: 'left', width: 20 }}></th>
                          <th style={{ padding: '2px 4px', textAlign: 'left' }}>板块 (L1)</th>
                          <th style={{ padding: '2px 4px', textAlign: 'right' }}>主力净流入</th>
                          <th style={{ padding: '2px 4px', textAlign: 'right' }}>涨跌幅</th>
                          <th style={{ padding: '2px 4px', textAlign: 'right', width: 30 }}>⭐</th>
                        </tr>
                      </thead>
                      <tbody>
                        {sectorTree.map((l1: any) => {
                          const l1Marked = markedSectors.has(l1.code)
                          const depth = l1.code === 'OTHER' ? 0 : 1
                          return (
                            <React.Fragment key={l1.code}>
                              <tr style={{ cursor: 'pointer', borderBottom: '1px solid #f0f0f0', background: l1Marked ? '#fff7e6' : '#fafafa' }}>
                                <td style={{ padding: '2px 4px', textAlign: 'center', fontSize: 10, color: '#999' }}>
                                  {l1.children?.length || ''}
                                </td>
                                <td style={{ padding: '2px 4px', fontWeight: 600, paddingLeft: depth * 16 + 4 }}
                                  onClick={() => toggleMarkSector(l1.code)}>
                                  {l1.name}
                                </td>
                                <td style={{ padding: '2px 4px', textAlign: 'right', color: l1.mainNet >= 0 ? '#f5222d' : '#52c41a' }}>
                                  {l1.mainNet ? `${(l1.mainNet / 1e8).toFixed(2)}亿` : '-'}
                                </td>
                                <td style={{ padding: '2px 4px', textAlign: 'right', color: l1.changePct >= 0 ? '#f5222d' : '#52c41a' }}>
                                  {l1.changePct != null ? `${l1.changePct >= 0 ? '+' : ''}${l1.changePct.toFixed(2)}%` : '-'}
                                </td>
                                <td style={{ padding: '2px 4px', textAlign: 'center' }}
                                  onClick={() => toggleMarkSector(l1.code)}>
                                  {l1Marked ? <StarFilled style={{ color: '#faad14', fontSize: 12 }} /> : <StarOutlined style={{ color: '#d9d9d9', fontSize: 12 }} />}
                                </td>
                              </tr>
                              {l1.children?.map((l2: any) => {
                                const l2Marked = markedSectors.has(l2.code)
                                return (
                                  <tr key={l2.code}
                                    style={{ borderBottom: '1px solid #f0f0f0', background: l2Marked ? '#fff7e6' : 'transparent' }}>
                                    <td style={{ padding: '2px 4px' }}></td>
                                    <td style={{ padding: '2px 4px', paddingLeft: 24 }}
                                      onClick={() => toggleMarkSector(l2.code)}>
                                      {l2.name}
                                      {l2.leader && (
                                        <Tag color="gold" style={{ marginLeft: 6, fontSize: 10, cursor: 'pointer' }}
                                          onClick={(e) => { e.stopPropagation(); handleSelectLeaderStock(l2.leader) }}>
                                          龙头 {l2.leader.name}
                                        </Tag>
                                      )}
                                    </td>
                                    <td style={{ padding: '2px 4px', textAlign: 'right', color: l2.mainNet >= 0 ? '#f5222d' : '#52c41a' }}>
                                      {(l2.mainNet / 1e8).toFixed(2)}亿
                                    </td>
                                    <td style={{ padding: '2px 4px', textAlign: 'right', color: l2.changePct >= 0 ? '#f5222d' : '#52c41a' }}>
                                      {l2.changePct >= 0 ? '+' : ''}{l2.changePct?.toFixed(2)}%
                                    </td>
                                    <td style={{ padding: '2px 4px', textAlign: 'center' }}
                                      onClick={() => toggleMarkSector(l2.code)}>
                                      {l2Marked ? <StarFilled style={{ color: '#faad14', fontSize: 12 }} /> : <StarOutlined style={{ color: '#d9d9d9', fontSize: 12 }} />}
                                    </td>
                                  </tr>
                                )
                              })}
                            </React.Fragment>
                          )
                        })}
                      </tbody>
                    </table>
                  ) : (
                    displaySectors.length > 0 && (
                      // fallback flat table when tree is unavailable (historical dates)
                      <table style={{ width: '100%', fontSize: 11, borderCollapse: 'collapse' }}>
                        <thead>
                          <tr style={{ background: '#fafafa', position: 'sticky', top: 0 }}>
                            <th style={{ padding: '2px 4px', width: 55, textAlign: 'center' }}>排名</th>
                            <th style={{ padding: '2px 4px', textAlign: 'left' }}>板块</th>
                            <th style={{ padding: '2px 4px', textAlign: 'right' }}>主力净流入</th>
                            <th style={{ padding: '2px 4px', textAlign: 'right' }}>占比</th>
                            <th style={{ padding: '2px 4px', textAlign: 'right' }}>涨跌幅</th>
                            <th style={{ padding: '2px 4px', textAlign: 'center', width: 30 }}>⭐</th>
                          </tr>
                        </thead>
                        <tbody>
                          {displaySectors.slice(0, 50).map((s: any) => {
                            const rank = sortedSectors.indexOf(s) + 1
                            const marked = markedSectors.has(s.code)
                            return (
                              <tr key={s.code}
                                style={{ cursor: 'pointer', borderBottom: '1px solid #f0f0f0', background: marked ? '#fff7e6' : 'transparent' }}>
                                <td style={{ padding: '2px 4px', textAlign: 'center', color: '#999', fontSize: 10 }}>{rank}/{sectors.length}</td>
                                <td style={{ padding: '2px 4px' }} onClick={() => toggleMarkSector(s.code)}>{s.name}</td>
                                <td style={{ padding: '2px 4px', textAlign: 'right', color: s.mainNet >= 0 ? '#f5222d' : '#52c41a' }}>
                                  {(s.mainNet / 1e8).toFixed(2)}亿
                                </td>
                                <td style={{ padding: '2px 4px', textAlign: 'right' }}>{s.mainRatio?.toFixed(1)}%</td>
                                <td style={{ padding: '2px 4px', textAlign: 'right', color: s.changePct >= 0 ? '#f5222d' : '#52c41a' }}>
                                  {s.changePct >= 0 ? '+' : ''}{s.changePct?.toFixed(2)}%
                                </td>
                                <td style={{ padding: '2px 4px', textAlign: 'center' }}
                                  onClick={() => toggleMarkSector(s.code)}>
                                  {marked ? <StarFilled style={{ color: '#faad14', fontSize: 12 }} /> : <StarOutlined style={{ color: '#d9d9d9', fontSize: 12 }} />}
                                </td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    )
                  )}
                </div>
              )}
            </Card>

            {/* Stock-specific section */}
            {searchedStock && (
              <Card
                title={`${searchedStock.name}(${searchedStock.code})`}
                size="small" style={{ marginBottom: 12 }}
                extra={
                  <Space size={4}>
                    <Text type="secondary" style={{ fontSize: 11 }}>天数:</Text>
                    <Slider
                      min={5} max={120} step={5} value={mfDays}
                      onChange={(v) => { setMfDays(v); if (searchedStock) handleSelectSearchResult(searchedStock) }}
                      style={{ width: 100, display: 'inline-block' }}
                      tooltip={{ formatter: (v) => `${v}天` }}
                    />
                  </Space>
                }
              >
                {stockRealtime && (
                  <Row gutter={12} style={{ marginBottom: 12 }}>
                    <Col span={6}>
                      <Statistic title="当前价" value={stockRealtime.price}
                        valueStyle={{ color: stockRealtime.changePercent >= 0 ? '#f5222d' : '#52c41a', fontSize: 18 }}
                        suffix={<span style={{ fontSize: 12 }}>{stockRealtime.changePercent >= 0 ? '+' : ''}{stockRealtime.changePercent?.toFixed(2)}%</span>}
                      />
                    </Col>
                    <Col span={6}><Statistic title="成交额" value={(stockRealtime.amount / 1e8).toFixed(1)} suffix="亿" /></Col>
                    <Col span={6}><Statistic title="成交量" value={(stockRealtime.volume / 1e4).toFixed(0)} suffix="万手" /></Col>
                    <Col span={6}>
                      <Statistic title="振幅" value={stockRealtime.high > 0 && stockRealtime.low > 0
                        ? `${((stockRealtime.high - stockRealtime.low) / stockRealtime.prevClose * 100).toFixed(2)}%` : '-'} />
                    </Col>
                  </Row>
                )}
                {stockMoneyFlow.length > 0 ? (
                  <div style={{ height: 200 }}>
                    <Column
                      data={stockMoneyFlow.map((d: any) => ({ ...d, _t: d.mainNet >= 0 ? '流入' : '流出' }))}
                      xField="date" yField="mainNet" height={200} colorField="_t"
                      scale={{ color: { domain: ['流入', '流出'], range: ['#f5222d', '#52c41a'] } }}
                      label={{ formatter: (v: any) => `${(v.mainNet / 1e4).toFixed(0)}万`, style: { fontSize: 9 } }}
                      xAxis={{ label: { autoRotate: true, style: { fontSize: 10 } } }}
                      tooltip={{ channel: 'y', valueFormatter: (v: number) => `${(v / 1e8).toFixed(2)}亿` }}
                    />
                  </div>
                ) : <Empty description="暂无资金流向数据" />}
              </Card>
            )}
          </Col>

          {/* Right: AI Analysis */}
          <Col xs={24} lg={12}>
            <Card
              title={<Space><RobotOutlined /> AI分析</Space>}
              size="small"
              extra={
                <Space size={4}>
                  <Button size="small" type={aiAnalysisScope === 'market' ? 'primary' : 'default'}
                    onClick={() => setAiAnalysisScope('market')}>市场分析</Button>
                  <Button size="small" type={aiAnalysisScope === 'self' ? 'primary' : 'default'}
                    icon={<StarOutlined />}
                    onClick={() => { setAiAnalysisScope('self'); loadSelfSelectData() }}>自选股分析</Button>
                  <Button type="primary" size="small" loading={aiLoading} onClick={handleAIAnalysis}>生成分析</Button>
                </Space>
              }
            >
              <div style={{ minHeight: 300, maxHeight: 'calc(100vh - 280px)', overflowY: 'auto' }}>
                {aiResult ? (
                  <div className="ai-message" style={{ fontSize: 14 }}>
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{aiResult}</ReactMarkdown>
                  </div>
                ) : (
                  <Text type="secondary">选择分析范围（市场/自选股），然后点击"生成分析"获取AI综合解读</Text>
                )}
              </div>
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  )
}
