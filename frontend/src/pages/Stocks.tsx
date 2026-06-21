import { useState, useCallback } from 'react'
import {
  Input, Table, Button, Tag, Space, Card, Row, Col, Statistic, Tabs, message, Spin, Select, Typography
} from 'antd'
import {
  SearchOutlined, PlusOutlined, StockOutlined, BarChartOutlined, RobotOutlined, FundOutlined
} from '@ant-design/icons'
import {
  SearchStock, GetRealtimeData, GetKLineData, GetTechnicalAnalysis,
  AddSelfSelectStock, GetGroups, GetAIAnalysis, GetTimeSharingData, GetMoneyFlow,
  GetStockIndustry
} from '../api/bridge'
import { Stock, Line } from '@ant-design/charts'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import ModelIndicator from '../components/ModelIndicator'

const { Text, Title } = Typography

function getExchangeAbbr(market: string, code: string): string {
  if (market === 'SH') return '沪'
  if (market === 'SZ') return '深'
  if (market === 'BJ') return '京'
  if (code?.startsWith('6')) return '沪'
  if (code?.startsWith('0') || code?.startsWith('3')) return '深'
  return market || '-'
}

function getBoardType(code: string): string {
  if (code?.startsWith('688') || code?.startsWith('689')) return '科创板'
  if (code?.startsWith('30')) return '创业板'
  if (code?.startsWith('60')) return '主板'
  if (code?.startsWith('00')) return '主板'
  if (code?.startsWith('002')) return '中小板'
  if (code?.startsWith('8')) return '新三板'
  if (code?.startsWith('4')) return '新三板'
  return 'A股'
}

export default function Stocks() {
  const [keyword, setKeyword] = useState('')
  const [searchResults, setSearchResults] = useState<any[]>([])
  const [searchLoading, setSearchLoading] = useState(false)
  const [selectedStock, setSelectedStock] = useState<any>(null)
  const [realtime, setRealtime] = useState<any>(null)
  const [techReport, setTechReport] = useState<any>(null)
  const [aiAnalysis, setAiAnalysis] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [loading, setLoading] = useState(false)
  const [groups, setGroups] = useState<any[]>([])
  const [activeTab, setActiveTab] = useState('tech')
  const [klineData, setKlineData] = useState<any[]>([])
  const [timeSharingData, setTimeSharingData] = useState<any[]>([])
  const [moneyFlow, setMoneyFlow] = useState<any[]>([])

  const doSearch = useCallback(async () => {
    if (!keyword.trim()) return
    setSearchLoading(true)
    const results = await SearchStock(keyword.trim())
    setSearchResults(results || [])
    setSearchLoading(false)
  }, [keyword])

  const [industry, setIndustry] = useState('')

  const selectStock = async (stock: any) => {
    setSelectedStock(stock)
    setLoading(true)
    setTechReport(null)
    setAiAnalysis('')
    setActiveTab('tech')
    setKlineData([])
    setTimeSharingData([])
    setMoneyFlow([])
    setIndustry('')

    const code = stock.fullCode || stock.code
    const [rt, tech, kline, ts, mf, ind] = await Promise.all([
      GetRealtimeData([code]),
      GetTechnicalAnalysis(code),
      GetKLineData(code, 120),
      GetTimeSharingData(code),
      GetMoneyFlow(code, 10),
      GetStockIndustry(code),
    ])

    if (rt) setRealtime(rt[code])
    setTechReport(tech)
    setKlineData((kline || []).map((d: any) => ({ ...d, trend: d.close > d.open ? 1 : d.close < d.open ? -1 : 0 })))
    setTimeSharingData(ts || [])
    setMoneyFlow(mf || [])
    setIndustry(ind || stock.industry || '')
    setLoading(false)
  }

  const handleAdd = async (stock: any) => {
    const code = stock.fullCode || stock.code
    const grps = await GetGroups()
    const defaultGroup = grps?.[0]?.name || '自选'
    const res = await AddSelfSelectStock(code, stock.name, defaultGroup)
    if (res === 'ok') {
      message.success(`已将 ${stock.name} 加入自选`)
    } else {
      message.warning(res)
    }
  }

  const handleAIAnalysis = async () => {
    if (!selectedStock) return
    setActiveTab('ai')
    setAiLoading(true)
    setAiAnalysis('分析中...')
    try {
      const result = await GetAIAnalysis(selectedStock.fullCode || selectedStock.code)
      setAiAnalysis(result)
    } finally {
      setAiLoading(false)
    }
  }

  const columns = [
    {
      title: '代码', dataIndex: 'code', key: 'code', width: 90,
    },
    {
      title: '名称', dataIndex: 'name', key: 'name', width: 110,
    },
    {
      title: '交易所', key: 'exchange', width: 60,
      render: (_: any, r: any) => <Tag>{getExchangeAbbr(r.market, r.code)}</Tag>,
    },
    {
      title: '板块', key: 'board', width: 70,
      render: (_: any, r: any) => <Tag color="blue">{getBoardType(r.code)}</Tag>,
    },
    {
      title: '行业', dataIndex: 'industry', key: 'industry', width: 100,
      render: (v: string) => v ? <Tag color="cyan">{v}</Tag> : '-',
    },
    {
      title: '类型', dataIndex: 'type', key: 'type', width: 60,
      render: (v: string) => <Tag color="geekblue">{v}</Tag>,
    },
    {
      title: '市场', dataIndex: 'market', key: 'market', width: 60,
      render: (v: string) => <Tag>{v}</Tag>,
    },
    {
      title: '操作', key: 'action', width: 130,
      render: (_: any, record: any) => (
        <Space size={4}>
          <Button type="primary" size="small" onClick={() => selectStock(record)}>详情</Button>
          <Button size="small" icon={<PlusOutlined />} onClick={() => handleAdd(record)}>自选</Button>
        </Space>
      ),
    },
  ]

  return (
    <div style={{ height: 'calc(100vh - 104px)', overflowY: 'auto' }}>
      <Title level={4} style={{ marginBottom: 16 }}>股票搜索</Title>

      <Input.Search
        placeholder="输入股票名称或代码搜索 (如: 贵州茅台, 600519)"
        enterButton={<><SearchOutlined /> 搜索</>}
        size="large"
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
        onSearch={doSearch}
        loading={searchLoading}
        style={{ marginBottom: 16 }}
      />

      {searchResults.length > 0 && (
        <Card size="small" title={`搜索结果 (${searchResults.length}条)`} style={{ marginBottom: 16 }}>
          <Table
            dataSource={searchResults}
            columns={columns}
            rowKey="code"
            size="small"
            pagination={{ pageSize: 10, showSizeChanger: false }}
          />
        </Card>
      )}

      {selectedStock && (
        <div>
          <Title level={5}>
            <StockOutlined /> {selectedStock.name} ({selectedStock.code})
            <Tag style={{ marginLeft: 8 }}>{getExchangeAbbr(selectedStock.market, selectedStock.code)}</Tag>
            <Tag color="blue">{getBoardType(selectedStock.code)}</Tag>
            {industry && <Tag color="cyan">{industry}</Tag>}
          </Title>

          {realtime && (
            <Row gutter={8} style={{ marginBottom: 12 }}>
              <Col span={3}><Card size="small"><Statistic title="现价" value={realtime.price} suffix={realtime.changePercent > 0 ? '↑' : '↓'} valueStyle={{ color: realtime.changePercent > 0 ? '#f5222d' : '#52c41a' }} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="涨幅" value={realtime.changePercent} precision={2} suffix="%" valueStyle={{ color: realtime.changePercent > 0 ? '#f5222d' : '#52c41a' }} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="开盘" value={realtime.open} precision={2} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="最高" value={realtime.high} precision={2} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="最低" value={realtime.low} precision={2} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="昨收" value={realtime.prevClose} precision={2} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="成交量" value={realtime.volume} /></Card></Col>
              <Col span={3}><Card size="small"><Statistic title="成交额" value={(realtime.amount / 1e8).toFixed(2)} suffix="亿" /></Card></Col>
            </Row>
          )}

          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            items={[
              {
                key: 'tech',
                label: <><BarChartOutlined /> 技术分析</>,
                children: loading ? <Spin style={{ display: 'block', margin: '40px auto' }} /> : (
                  techReport ? <TechReport report={techReport} /> : <Text type="secondary">暂无数据</Text>
                ),
              },
              {
                key: 'kline',
                label: <><StockOutlined /> K线图</>,
                children: klineData.length > 0 ? (
                  <div style={{ height: 360 }}>
                    <Stock
                      data={klineData}
                      xField="date"
                      yField={['open', 'close', 'high', 'low']}
                      height={360}
                      colorField="trend"
                    />
                  </div>
                ) : <Text type="secondary">暂无K线数据</Text>,
              },
              {
                key: 'timeshare',
                label: <><FundOutlined /> 分时图</>,
                children: timeSharingData.length > 0 ? (
                  <div style={{ height: 360 }}>
                    <Line
                      data={timeSharingData}
                      xField="date"
                      yField="close"
                      height={360}
                      smooth={false}
                      color="#1677ff"
                    />
                  </div>
                ) : <Text type="secondary">暂无分时数据</Text>,
              },
              {
                key: 'moneyflow',
                label: <><FundOutlined /> 资金流向</>,
                children: moneyFlow.length > 0 ? (
                  <Table
                    dataSource={moneyFlow}
                    rowKey="date"
                    size="small"
                    columns={[
                      { title: '日期', dataIndex: 'date', key: 'date' },
                      {
                        title: '主力净流入', dataIndex: 'mainNet', key: 'mainNet',
                        render: (v: number) => {
                          const val = v / 1e8;
                          return <Text style={{ color: val >= 0 ? '#f5222d' : '#52c41a', fontWeight: 600 }}>{val >= 0 ? '+' : ''}{val.toFixed(2)}亿</Text>;
                        },
                        sorter: (a: any, b: any) => a.mainNet - b.mainNet,
                      },
                      { title: '占比', dataIndex: 'mainRatio', key: 'mainRatio', render: (v: number) => v ? <Text style={{ color: v >= 0 ? '#f5222d' : '#52c41a' }}>{v >= 0 ? '+' : ''}{v.toFixed(1)}%</Text> : '-' },
                      { title: '超大单', dataIndex: 'superLargeNet', key: 'superLargeNet', render: (v: number) => `${(v / 1e4).toFixed(0)}万` },
                      { title: '大单', dataIndex: 'largeNet', key: 'largeNet', render: (v: number) => `${(v / 1e4).toFixed(0)}万` },
                    ]}
                  />
                ) : <Text type="secondary">暂无数据</Text>,
              },
              {
                key: 'ai',
                label: <><RobotOutlined /> AI分析</>,
                children: (
                  <div>
                    <Space style={{ marginBottom: 12 }} size={8}>
                      <Button type="primary" icon={<RobotOutlined />} loading={aiLoading} onClick={handleAIAnalysis}>
                        AI智能分析
                      </Button>
                      <ModelIndicator compact />
                    </Space>
                    <Card>
                      <div style={{ lineHeight: 1.8 }}>{aiAnalysis ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{aiAnalysis}</ReactMarkdown> : '点击按钮获取AI分析建议'}</div>
                    </Card>
                  </div>
                ),
              },
            ]}
          />
        </div>
      )}
    </div>
  )
}

function TechReport({ report }: { report: any }) {
  if (!report) return null

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}>
          <Card size="small">
            <Statistic
              title="操作建议"
              value={report.suggestion}
              valueStyle={{ color: report.suggestion === '买入' ? '#f5222d' : report.suggestion === '卖出' ? '#52c41a' : '#faad14', fontSize: 18 }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="置信度" value={report.confidence} suffix="%" /></Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="支撑位" value={report.support} precision={2} /></Card>
        </Col>
        <Col span={6}>
          <Card size="small"><Statistic title="压力位" value={report.resistance} precision={2} /></Card>
        </Col>
      </Row>

      <Card size="small" title="指标信号" style={{ marginBottom: 12 }}>
        <Row gutter={[8, 8]}>
          {report.indicators?.map((ind: any, i: number) => (
            <Col key={i}>
              <Tag color={ind.signal === 'buy' ? 'green' : ind.signal === 'sell' ? 'red' : 'default'}>
                {ind.name}: {ind.value} ({ind.signal === 'buy' ? '买入' : ind.signal === 'sell' ? '卖出' : '中性'})
              </Tag>
            </Col>
          ))}
        </Row>
      </Card>

      <Card size="small" title="综合评估">
        <Text>{report.summary}</Text>
      </Card>
    </div>
  )
}
