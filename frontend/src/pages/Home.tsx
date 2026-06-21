import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import {
  Card, Row, Col, Statistic, Spin, Button, Select, Typography, Space, Tag, message, Tabs, Descriptions, Table, Tooltip, Empty, InputNumber, Modal, Divider, Radio
} from 'antd'
import {
  ReloadOutlined, ArrowUpOutlined, ArrowDownOutlined, MinusOutlined,
  StockOutlined, BarChartOutlined, FundOutlined, RobotOutlined, EditOutlined, LineChartOutlined
} from '@ant-design/icons'
import { Stock, Line, Column } from '@ant-design/charts'
import {
  GetGroups, GetSelfSelectStocks, RefreshAllSelfSelect, GetRefreshInterval,
  RemoveSelfSelectStock, MoveStockToGroup,
  GetKLineData, GetTechnicalAnalysis, GetTimeSharingData, GetMoneyFlow, GetAIAnalysis,
  GetPosition, SavePosition, DeletePosition
} from '../api/bridge'
import GroupManager from '../components/GroupManager'
import dayjs from 'dayjs'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import ModelIndicator from '../components/ModelIndicator'

const { Text, Title } = Typography

function isMarketOpen(): boolean {
  const now = new Date()
  const dow = now.getDay()
  if (dow === 0 || dow === 6) return false
  const h = now.getHours()
  const m = now.getMinutes()
  const t = h * 100 + m
  return (t >= 930 && t <= 1130) || (t >= 1300 && t <= 1500)
}

function getExchangeName(code: string): string {
  if (code?.startsWith('sh') || code?.startsWith('6')) return '沪'
  if (code?.startsWith('sz') || code?.startsWith('0') || code?.startsWith('3')) return '深'
  if (code?.startsWith('bj') || code?.startsWith('8')) return '京'
  return '-'
}

export default function Home() {
  const [stocks, setStocks] = useState<any[]>([])
  const [groups, setGroups] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [selectedGroup, setSelectedGroup] = useState<string>('all')
  const [refreshInterval, setRefreshInterval] = useState(2)
  const [webSearch, setWebSearch] = useState(false)
  const [lastRefreshTime, setLastRefreshTime] = useState<number>(Date.now())
  const [hasRefreshedOnce, setHasRefreshedOnce] = useState(false)
  const timerRef = useRef<any>(null)
  const loadReqId = useRef(0)
  const marketOpen = useMemo(() => isMarketOpen(), [loading])

  // selected stock detail
  const [selectedCode, setSelectedCode] = useState<string>('')
  const [selectedInfo, setSelectedInfo] = useState<any>(null)
  const [klineData, setKlineData] = useState<any[]>([])
  const [timeSharingData, setTimeSharingData] = useState<any[]>([])
  const [techReport, setTechReport] = useState<any>(null)
  const [moneyFlow, setMoneyFlow] = useState<any[]>([])
  const [aiAnalysis, setAiAnalysis] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [chartTab, setChartTab] = useState('kline')
  const [detailLoading, setDetailLoading] = useState(false)

  // position management
  const [position, setPosition] = useState<any>(null)
  const [positionModalOpen, setPositionModalOpen] = useState(false)
  const [positionForm, setPositionForm] = useState<any>({})
  const [moneyFlowChartOpen, setMoneyFlowChartOpen] = useState(false)
  const [moneyFlowDays, setMoneyFlowDays] = useState(30)

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [result, grps, iv] = await Promise.all([
        RefreshAllSelfSelect(),
        GetGroups(),
        GetRefreshInterval(),
      ])
      setStocks(result || [])
      setGroups(grps || [])
      setRefreshInterval(iv ?? 2)
      setLastRefreshTime(Date.now())
      setHasRefreshedOnce(true)
    } catch (e) {
      console.error('loadData failed:', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadData()
  }, [loadData])

  useEffect(() => {
    if (refreshInterval > 0 && marketOpen) {
      timerRef.current = setInterval(loadData, Math.max(refreshInterval * 1000, 10))
      return () => clearInterval(timerRef.current)
    }
  }, [refreshInterval, loadData, marketOpen])

  const handleSelectStock = async (item: any) => {
    const code = item.info?.fullCode || item.info?.code
    if (!code || code === selectedCode) return
    const reqId = ++loadReqId.current
    setSelectedCode(code)
    setSelectedInfo(item)
    setDetailLoading(true)
    setAiAnalysis('')
    setChartTab('kline')

    try {
      const [kline, timeshare, tech, mflow, pos] = await Promise.all([
        GetKLineData(code, 120),
        GetTimeSharingData(code),
        GetTechnicalAnalysis(code),
        GetMoneyFlow(code, moneyFlowDays),
        GetPosition(item.info?.code || ''),
      ])
      if (loadReqId.current !== reqId) return // stale response
      setKlineData((kline || []).map((d: any) => ({ ...d, trend: d.close > d.open ? 1 : d.close < d.open ? -1 : 0 })))
      setTimeSharingData(timeshare || [])
      setTechReport(tech)
      setMoneyFlow((mflow || []).reverse())
      setPosition(pos)
    } catch (e) {
      console.error('handleSelectStock error:', e)
    } finally {
      if (loadReqId.current === reqId) {
        setDetailLoading(false)
      }
    }
  }

  const handleAIAnalysis = async () => {
    if (!selectedCode) return
    setAiLoading(true)
    setAiAnalysis('分析中...')
    try {
      const prefix = webSearch ? '[联网搜索] ' : ''
      const result = await GetAIAnalysis(prefix + selectedCode)
      setAiAnalysis(result)
    } finally {
      setAiLoading(false)
    }
  }

  const handleRemove = async (code: string) => {
    const res = await RemoveSelfSelectStock(code)
    if (res === 'ok') {
      message.success('已删除')
      if (selectedCode === code) {
        setSelectedCode('')
        setSelectedInfo(null)
      }
      loadData()
    } else {
      message.error(res)
    }
  }

  const handleMoveGroup = async (code: string, group: string) => {
    const res = await MoveStockToGroup(code, group)
    if (res === 'ok') {
      message.success('已移动')
      loadData()
    } else {
      message.error(res)
    }
  }

  const handleSavePosition = async () => {
    const pos = {
      code: positionForm.code,
      name: positionForm.name,
      quantity: positionForm.quantity || 0,
      costPrice: positionForm.costPrice || 0,
      targetPrice: positionForm.targetPrice || 0,
      stopLoss: positionForm.stopLoss || 0,
      notes: positionForm.notes || '',
    }
    const res = await SavePosition(pos)
    if (res === 'ok') {
      message.success('持仓信息已保存')
      setPosition(pos)
      setPositionModalOpen(false)
    } else {
      message.error(res)
    }
  }

  const handleDeletePosition = async () => {
    const res = await DeletePosition(positionForm.code)
    if (res === 'ok') {
      message.success('持仓已删除')
      setPosition(null)
      setPositionModalOpen(false)
    } else {
      message.error(res)
    }
  }

  const openPositionModal = () => {
    const code = selectedInfo?.info?.code || ''
    const name = selectedInfo?.info?.name || ''
    setPositionForm(position || { code, name, quantity: 0, costPrice: 0, targetPrice: 0, stopLoss: 0, notes: '' })
    setPositionModalOpen(true)
  }

  const filteredStocks = selectedGroup === 'all'
    ? stocks
    : stocks.filter((s: any) => s.info?.group === selectedGroup)

  const groupedStocks: Record<string, any[]> = {}
  stocks.forEach((s: any) => {
    const g = s.info?.group || '自选'
    if (!groupedStocks[g]) groupedStocks[g] = []
    groupedStocks[g].push(s)
  })

  const refreshTimeDisplay = useMemo(() => {
    if (!hasRefreshedOnce) return { text: 'NA', fresh: false, stale: true }
    if (!marketOpen) return { text: '', fresh: false, stale: false, closed: true }
    const elapsed = (Date.now() - lastRefreshTime) / 1000
    if (elapsed < 1) return { text: '最新', fresh: true, stale: false }
    const dt = new Date(lastRefreshTime)
    const ts = `${dt.getFullYear()}-${String(dt.getMonth() + 1).padStart(2, '0')}-${String(dt.getDate()).padStart(2, '0')} ${String(dt.getHours()).padStart(2, '0')}:${String(dt.getMinutes()).padStart(2, '0')}:${String(dt.getSeconds()).padStart(2, '0')}`
    return { text: ts, fresh: false, stale: true }
  }, [lastRefreshTime, loading, marketOpen, hasRefreshedOnce])

  if (loading && stocks.length === 0) {
    return <div style={{ textAlign: 'center', padding: 80 }}><Spin size="large" /></div>
  }

  const displayList = selectedGroup === 'all' ? stocks : filteredStocks

  return (
    <Row gutter={16} style={{ height: 'calc(100vh - 104px)' }}>
      <Col span={8} className="dashboard-left">
        <div style={{ marginBottom: 12 }}>
          <Row justify="space-between" align="middle">
            <Col>
              <Space size={8}>
                <Title level={5} style={{ margin: 0 }}>自选股票池</Title>
                {marketOpen ? (
                  <Tag color="green" style={{ fontSize: 10, lineHeight: '16px' }}>开市</Tag>
                ) : (
                  <Tag color="warning" style={{ fontSize: 10, lineHeight: '16px' }}>闭市</Tag>
                )}
                {refreshTimeDisplay.closed ? null : refreshTimeDisplay.fresh ? (
                  <Tag color="green" style={{ fontSize: 10, lineHeight: '16px' }}>最新</Tag>
                ) : refreshTimeDisplay.stale ? (
                  <Text className="refresh-time-stale">{refreshTimeDisplay.text}</Text>
                ) : null}
              </Space>
            </Col>
            <Col>
              <Space size={4}>
                <Radio.Group
                  size="small"
                  value={selectedGroup}
                  onChange={(e) => setSelectedGroup(e.target.value)}
                  optionType="button"
                  buttonStyle="solid"
                >
                  <Radio.Button value="all">全部</Radio.Button>
                  {groups.map((g: any) => (
                    <Radio.Button key={g.name} value={g.name}>{g.name}</Radio.Button>
                  ))}
                </Radio.Group>
                <GroupManager onRefresh={loadData} />
                <Tooltip title="手动刷新">
                  <Button size="small" icon={<ReloadOutlined />} onClick={loadData} loading={loading} />
                </Tooltip>
              </Space>
            </Col>
          </Row>
        </div>
        {displayList.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
            <Text>暂无自选股，前往「股票搜索」添加</Text>
          </div>
        ) : (
          <div>
            {displayList.map((item: any) => {
              const info = item.info || {}
              const m = item.market
              const isSelected = selectedCode === (info.fullCode || info.code)
              return (
                <div
                  key={info.code}
                  className={`stock-list-item${isSelected ? ' selected' : ''}`}
                  onClick={() => handleSelectStock(item)}
                >
                  <Row justify="space-between" align="middle">
                    <Col>
                      <Space size={4}>
                        <Text strong style={{ fontSize: 13 }}>{info.name}</Text>
                        <Tag style={{ fontSize: 10, lineHeight: '14px', transform: 'scale(0.85)' }}>
                          {getExchangeName(info.code)}
                        </Tag>
                      </Space>
                      <div><Text type="secondary" style={{ fontSize: 11 }}>{info.code}</Text></div>
                    </Col>
                    {m ? (
                      <Col style={{ textAlign: 'right' }}>
                        <div style={{
                          fontSize: 16, fontWeight: 600,
                          color: m.changePercent > 0 ? '#f5222d' : m.changePercent < 0 ? '#52c41a' : '#333'
                        }}>
                          {m.price?.toFixed(2)}
                        </div>
                        <Space size={2}>
                          {m.changePercent > 0 ? <ArrowUpOutlined style={{ color: '#f5222d', fontSize: 11 }} /> :
                           m.changePercent < 0 ? <ArrowDownOutlined style={{ color: '#52c41a', fontSize: 11 }} /> :
                           <MinusOutlined style={{ color: '#999', fontSize: 11 }} />}
                          <Text style={{
                            color: m.changePercent > 0 ? '#f5222d' : m.changePercent < 0 ? '#52c41a' : '#999',
                            fontSize: 12
                          }}>
                            {m.changePercent > 0 ? '+' : ''}{m.changePercent?.toFixed(2)}%
                          </Text>
                        </Space>
                      </Col>
                    ) : (
                      <Col><Tag color="default">加载中</Tag></Col>
                    )}
                  </Row>
                </div>
              )
            })}
          </div>
        )}
      </Col>

      <Col span={16} className="dashboard-right">
        {!selectedCode ? (
          <div style={{ textAlign: 'center', padding: 80, color: '#999' }}>
            <StockOutlined style={{ fontSize: 48, marginBottom: 16 }} />
            <Title level={4} type="secondary">选择左侧自选股查看详情</Title>
            <Text>点击左侧股票列表中的股票，查看K线、分时、资金流向等详细分析</Text>
          </div>
        ) : detailLoading ? (
          <div style={{ textAlign: 'center', padding: 80 }}><Spin size="large" /></div>
        ) : (
          <div>
            {/* Stock info banner + position */}
            {selectedInfo?.market && (
              <div style={{ marginBottom: 12 }}>
                <Row gutter={8} align="middle" style={{ marginBottom: 8 }}>
                  <Col>
                    <Title level={4} style={{ margin: 0 }}>
                      {selectedInfo.info?.name}
                      <Text type="secondary" style={{ fontSize: 14, marginLeft: 8 }}>{selectedInfo.info?.code}</Text>
                      <Tag style={{ marginLeft: 6 }}>{getExchangeName(selectedInfo.info?.code)}</Tag>
                    </Title>
                  </Col>
                  <Col flex="auto" style={{ textAlign: 'right' }}>
                    <Space size="middle" wrap>
                      <Statistic title="现价" value={selectedInfo.market.price}
                        valueStyle={{ color: selectedInfo.market.changePercent > 0 ? '#f5222d' : '#52c41a', fontSize: 20 }}
                        suffix={<span style={{ fontSize: 13 }}>
                          {selectedInfo.market.changePercent > 0 ? '+' : ''}{selectedInfo.market.changePercent?.toFixed(2)}%
                        </span>}
                      />
                      <Statistic title="最高" value={selectedInfo.market.high} precision={2} />
                      <Statistic title="最低" value={selectedInfo.market.low} precision={2} />
                      <Statistic title="成交量" value={selectedInfo.market.volume} />
                      <Statistic title="成交额" value={(selectedInfo.market.amount / 1e8).toFixed(2)} suffix="亿" />
                    </Space>
                  </Col>
                  <Col>
                    <Button size="small" icon={<EditOutlined />} onClick={openPositionModal}>持仓</Button>
                  </Col>
                </Row>

                {position && position.quantity > 0 && (
                  <div className="position-card">
                    <Row gutter={16}>
                      <Col span={6}><div className="label">持仓数量</div><div className="value">{position.quantity}股</div></Col>
                      <Col span={6}><div className="label">成本价</div><div className="value">¥{position.costPrice?.toFixed(2)}</div></Col>
                      <Col span={6}>
                        <div className="label">浮动盈亏</div>
                        <div className="value" style={{
                          color: (selectedInfo.market.price - position.costPrice) * position.quantity >= 0 ? '#f5222d' : '#52c41a'
                        }}>
                          ¥{((selectedInfo.market.price - position.costPrice) * position.quantity).toFixed(2)}
                        </div>
                      </Col>
                      {position.targetPrice > 0 && (
                        <Col span={3}><div className="label">目标价</div><div className="value">¥{position.targetPrice.toFixed(2)}</div></Col>
                      )}
                      {position.stopLoss > 0 && (
                        <Col span={3}><div className="label">止损价</div><div className="value">¥{position.stopLoss.toFixed(2)}</div></Col>
                      )}
                    </Row>
                  </div>
                )}
              </div>
            )}

            {/* Charts */}
            <Card size="small" className="chart-card" style={{ marginBottom: 12 }}>
              <Tabs
                activeKey={chartTab}
                onChange={setChartTab}
                items={[
                  {
                    key: 'kline',
                    label: <><BarChartOutlined /> 日K线</>,
                    children: klineData.length > 0 ? (
                      <div style={{ height: 340 }}>
                        <Stock
                          key={`${selectedCode}-${position?.costPrice || 0}-${position?.targetPrice || 0}-${position?.stopLoss || 0}`}
                          data={klineData}
                          xField="date"
                          yField={['open', 'close', 'high', 'low']}
                          height={340}
                          colorField="trend"
                          slider={{ x: true }}
                          onReady={(chart: any) => {
                            try {
                              if (chart.annotation) {
                                chart.annotation().clear()
                              }
                            } catch (_) {}
                            try {
                              const lines: { value: number; label: string; color: string }[] = []
                              if (position?.costPrice > 0) {
                                lines.push({ value: position.costPrice, label: '成本', color: '#fa8c16' })
                              }
                              if (position?.targetPrice > 0) {
                                lines.push({ value: position.targetPrice, label: '目标', color: '#52c41a' })
                              }
                              if (position?.stopLoss > 0) {
                                lines.push({ value: position.stopLoss, label: '止损', color: '#999999' })
                              }
                              if (lines.length > 0 && typeof chart.annotation === 'function') {
                                lines.forEach(({ value, label, color }) => {
                                  try {
                                    chart.annotation()
                                      .line({
                                        top: true,
                                        start: ['min', value],
                                        end: ['max', value],
                                        style: { stroke: color, lineDash: [4, 4], lineWidth: 2 },
                                      })
                                    chart.annotation()
                                      .text({
                                        top: true,
                                        position: ['max', value],
                                        content: label,
                                        offsetY: -8,
                                        style: { fill: color, fontSize: 10, fontWeight: 600 },
                                      })
                                  } catch (_) {}
                                })
                                if (typeof chart.render === 'function') chart.render()
                              }
                            } catch (e) {
                              console.warn('K-line annotation skipped:', (e as Error).message)
                            }
                          }}
                        />
                      </div>
                    ) : <Empty description="暂无K线数据" />
                  },
                  {
                    key: 'timeshare',
                    label: <><FundOutlined /> 分时图</>,
                    children: timeSharingData.length > 0 ? (
                      <div style={{ height: 340 }}>
                        <Line
                          data={timeSharingData}
                          xField="date"
                          yField="close"
                          height={340}
                          smooth={false}
                          color="#1677ff"
                          slider={{ x: true }}
                        />
                      </div>
                    ) : <Empty description="暂无分时数据" />
                  },
                ]}
              />
            </Card>

            {/* Technical indicators + Money flow */}
            <Row gutter={12}>
              <Col span={12}>
                <Card size="small" title={<><BarChartOutlined /> 技术指标</>} style={{ marginBottom: 12 }}>
                  {techReport ? (
                    <>
                      <Descriptions size="small" column={1}>
                        <Descriptions.Item label="操作建议">
                          <Tag color={techReport.suggestion === '买入' ? 'red' : techReport.suggestion === '卖出' ? 'green' : 'gold'}>
                            {techReport.suggestion}
                          </Tag>
                        </Descriptions.Item>
                        <Descriptions.Item label="置信度">{techReport.confidence}%</Descriptions.Item>
                        <Descriptions.Item label="支撑位">{techReport.support?.toFixed(2)}</Descriptions.Item>
                        <Descriptions.Item label="压力位">{techReport.resistance?.toFixed(2)}</Descriptions.Item>
                      </Descriptions>
                      {techReport.indicators && (
                        <div style={{ marginTop: 8 }}>
                          <Space wrap size={4}>
                            {techReport.indicators.map((ind: any, i: number) => (
                              <Tag key={i} color={ind.signal === 'buy' ? 'green' : ind.signal === 'sell' ? 'red' : 'default'}>
                                {ind.name}: {ind.value}
                              </Tag>
                            ))}
                          </Space>
                        </div>
                      )}
                      {techReport.summary && (
                        <div style={{ marginTop: 8, fontSize: 13, color: '#666' }}>
                          <Text>{techReport.summary}</Text>
                        </div>
                      )}
                    </>
                  ) : <Text type="secondary">暂无技术分析数据</Text>}
                </Card>
              </Col>
              <Col span={12}>
                <Card size="small" title={<><FundOutlined /> 资金流向(近30日)</>}
                   extra={<Button size="small" icon={<BarChartOutlined />} onClick={() => setMoneyFlowChartOpen(true)}>柱状图</Button>}
                  style={{ marginBottom: 12 }}
                >
                  {moneyFlow.length > 0 ? (
                    <Table
                      dataSource={moneyFlow.slice(0, 10)}
                      rowKey="date"
                      size="small"
                      pagination={false}
                      columns={[
                        { title: '日期', dataIndex: 'date', key: 'date', width: 80 },
                        { title: '主力净流入', dataIndex: 'mainNet', key: 'mainNet', width: 100,
                          render: (v: number) => {
                            const val = v / 1e8;
                            return <Text style={{ color: val >= 0 ? '#f5222d' : '#52c41a', fontWeight: 600 }}>{val >= 0 ? '+' : ''}{val.toFixed(2)}亿</Text>;
                          }
                        },
                        { title: '占比', dataIndex: 'mainRatio', key: 'mainRatio', width: 60,
                          render: (v: number) => v ? <Text style={{ color: v >= 0 ? '#f5222d' : '#52c41a' }}>{v >= 0 ? '+' : ''}{v.toFixed(1)}%</Text> : '-',
                        },
                        { title: '超大单', dataIndex: 'superLargeNet', key: 'superLargeNet', width: 70,
                          render: (v: number) => `${(v / 1e4).toFixed(0)}万`,
                        },
                        { title: '大单', dataIndex: 'largeNet', key: 'largeNet', width: 70,
                          render: (v: number) => `${(v / 1e4).toFixed(0)}万`,
                        },
                      ]}
                    />
                  ) : <Text type="secondary">暂无资金流向数据</Text>}
                </Card>
              </Col>
            </Row>

            {/* AI Analysis */}
            <Card
              size="small"
              title={<><RobotOutlined /> AI分析解读</>}
              extra={<Space size={4}>
                      <ModelIndicator compact webSearch={webSearch} onWebSearchChange={setWebSearch} />
                <Button type="primary" size="small" loading={aiLoading} onClick={handleAIAnalysis}>AI分析</Button>
              </Space>}
            >
              <div style={{ fontSize: 13, minHeight: 40, lineHeight: 1.8 }}>
                {aiAnalysis ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{aiAnalysis}</ReactMarkdown> : '点击「AI智能分析」按钮获取对该股票的深度解读'}
              </div>
            </Card>
          </div>
        )}
      </Col>

      {/* Position Modal */}
      <Modal title="持仓管理" open={positionModalOpen} onCancel={() => setPositionModalOpen(false)}
        onOk={handleSavePosition} okText="保存" cancelText="取消"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Row gutter={12}>
            <Col span={12}>
              <Text type="secondary">股票代码</Text>
              <InputNumber style={{ width: '100%' }} value={positionForm.code} disabled />
            </Col>
            <Col span={12}>
              <Text type="secondary">股票名称</Text>
              <InputNumber style={{ width: '100%' }} value={positionForm.name} disabled />
            </Col>
          </Row>
          <Divider style={{ margin: '8px 0' }} />
          <Row gutter={12}>
            <Col span={8}>
              <Text type="secondary">持仓数量(股)</Text>
              <InputNumber style={{ width: '100%' }} min={0}
                value={positionForm.quantity}
                onChange={(v) => setPositionForm({ ...positionForm, quantity: v || 0 })}
              />
            </Col>
            <Col span={8}>
              <Text type="secondary">成本价(元)</Text>
              <InputNumber style={{ width: '100%' }} min={0} precision={2}
                value={positionForm.costPrice}
                onChange={(v) => setPositionForm({ ...positionForm, costPrice: v || 0 })}
              />
            </Col>
            <Col span={8}>
              <Text type="secondary">目标价(元)</Text>
              <InputNumber style={{ width: '100%' }} min={0} precision={2}
                value={positionForm.targetPrice}
                onChange={(v) => setPositionForm({ ...positionForm, targetPrice: v || 0 })}
              />
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Text type="secondary">止损价(元)</Text>
              <InputNumber style={{ width: '100%' }} min={0} precision={2}
                value={positionForm.stopLoss}
                onChange={(v) => setPositionForm({ ...positionForm, stopLoss: v || 0 })}
              />
            </Col>
          </Row>
          {position && position.quantity > 0 && (
            <Button danger onClick={handleDeletePosition}>清空持仓</Button>
          )}
        </Space>
      </Modal>

      {/* Money Flow Chart Modal */}
      <Modal title={`资金流向柱状图(近${moneyFlowDays}日)`} open={moneyFlowChartOpen}
        onCancel={() => setMoneyFlowChartOpen(false)} footer={null} width={700}
      >
        <div style={{ marginBottom: 12 }}>
          <Space>
            <span style={{ fontSize: 12, color: '#999' }}>天数:</span>
            {[10, 30, 60, 120].map(n => (
              <Button key={n} size="small" type={moneyFlowDays === n ? 'primary' : 'default'}
                onClick={async () => {
                  setMoneyFlowDays(n)
                  const mf = await GetMoneyFlow(selectedCode, n)
                  setMoneyFlow((mf || []).reverse())
                }}
              >{n}日</Button>
            ))}
          </Space>
        </div>
        {moneyFlow.length > 0 ? (
          <div style={{ height: 350 }}>
            <Column
              data={moneyFlow.map((d: any) => ({ ...d, _flow: d.mainNet >= 0 ? '流入' : '流出' }))}
              xField="date"
              yField="mainNet"
              height={350}
              colorField="_flow"
              scale={{
                color: { domain: ['流入', '流出'], range: ['#f5222d', '#52c41a'] },
              }}
              slider={{ x: true }}
              label={{
                formatter: (v: any) => `${((typeof v === 'number' ? v : 0) / 1e8).toFixed(1)}亿`,
                style: { fontSize: 10 },
              }}
            />
          </div>
        ) : <Empty description="暂无数据" />}
      </Modal>
    </Row>
  )
}
