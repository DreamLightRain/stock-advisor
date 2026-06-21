import { useState, useEffect } from 'react'
import { Card, Select, Typography, Space, Tag, Empty, Spin, Button, message, Alert, Collapse, Switch } from 'antd'
import { FileTextOutlined, RobotOutlined, SearchOutlined } from '@ant-design/icons'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { GetLogDates, GetLogModules, GetLogs, GetLogAIInterpretation } from '../api/bridge'
import ModelIndicator from '../components/ModelIndicator'

const { Title, Text } = Typography

const MODULE_LABELS: Record<string, string> = {
  system: '系统',
  data: '数据抓取',
  ai: 'AI调用',
  storage: '存储',
  analysis: '分析',
}

export default function Logs() {
  const [dates, setDates] = useState<string[]>([])
  const [selectedDate, setSelectedDate] = useState('')
  const [modules, setModules] = useState<string[]>([])
  const [selectedModule, setSelectedModule] = useState('')
  const [entries, setEntries] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [aiLoading, setAiLoading] = useState(false)
  const [aiResult, setAiResult] = useState('')

  // Load dates on mount
  useEffect(() => {
    GetLogDates().then((ds: string[]) => {
      setDates(ds || [])
      if (ds && ds.length > 0) setSelectedDate(ds[ds.length - 1])
    })
  }, [])

  // Load modules when date changes
  useEffect(() => {
    if (selectedDate) {
      GetLogModules(selectedDate).then((ms: string[]) => {
        setModules(ms || [])
        if (ms && ms.length > 0) setSelectedModule(ms[0])
      })
    }
  }, [selectedDate])

  // Load logs when date+module change
  useEffect(() => {
    if (selectedDate && selectedModule) {
      setLoading(true)
      GetLogs(selectedDate, selectedModule).then((es: any[]) => {
        setEntries(es || [])
        setLoading(false)
      })
    }
  }, [selectedDate, selectedModule])

  const handleAIInterpret = async () => {
    setAiLoading(true)
    setAiResult('')
    try {
      const result = await GetLogAIInterpretation(selectedDate, selectedModule)
      setAiResult(result || '无返回')
    } catch (e: any) {
      setAiResult(`AI分析失败: ${e.message}`)
    }
    setAiLoading(false)
  }

  const levelColor = (level: string) => {
    switch (level) {
      case 'ERROR': return 'red'
      case 'WARN': return 'orange'
      case 'INFO': return 'blue'
      case 'DEBUG': return 'default'
      default: return 'default'
    }
  }

  return (
    <div>
      <Title level={4}><FileTextOutlined /> 日志记录</Title>

      {/* Filters */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <Space wrap>
          <Text>日期:</Text>
          <Select
            value={selectedDate}
            onChange={setSelectedDate}
            style={{ width: 140 }}
            options={dates.map(d => ({ label: d, value: d }))}
          />
          <Text>模块:</Text>
          <Select
            value={selectedModule}
            onChange={setSelectedModule}
            style={{ width: 160 }}
            options={modules.map(m => ({ label: MODULE_LABELS[m] || m, value: m }))}
          />
          {modules.length > 0 && (
            <Text type="secondary" style={{ fontSize: 12 }}>
              共 {entries.length} 条记录
            </Text>
          )}
        </Space>
      </Card>

      <Spin spinning={loading}>
        {/* Log entries */}
        {entries.length > 0 ? (
          <div style={{ maxHeight: 'calc(100vh - 400px)', overflowY: 'auto', marginBottom: 16 }}>
            {entries.map((e: any, i: number) => (
              <div key={i} style={{
                padding: '4px 8px',
                borderBottom: '1px solid #f5f5f5',
                fontSize: 12,
                fontFamily: 'monospace',
                display: 'flex',
                gap: 8,
                background: e.level === 'ERROR' ? '#fff2f0' : e.level === 'WARN' ? '#fffbe6' : 'transparent',
              }}>
                <Text style={{ color: '#999', whiteSpace: 'nowrap', minWidth: 110 }}>{e.timestamp}</Text>
                <Tag color={levelColor(e.level)} style={{ margin: 0, fontSize: 10, lineHeight: '16px', minWidth: 50, textAlign: 'center' }}>
                  {e.level}
                </Tag>
                <Text style={{ flex: 1, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{e.message}</Text>
              </div>
            ))}
          </div>
        ) : (
          !loading && <Empty description={selectedDate && selectedModule ? '暂无日志' : '请选择日期和模块'} />
        )}

        {/* AI Interpretation */}
        <Card
          size="small"
          title={<Space><RobotOutlined /> AI日志解读</Space>}
          extra={
            <Space size={4}>
               <ModelIndicator compact />
              <Button type="primary" size="small" loading={aiLoading} onClick={handleAIInterpret}
                disabled={!selectedDate || !selectedModule || entries.length === 0}>
                分析日志
              </Button>
            </Space>
          }
        >
          {aiResult ? (
            <div className="ai-message" style={{ fontSize: 14 }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{aiResult}</ReactMarkdown>
            </div>
          ) : (
            <Text type="secondary">点击"分析日志"获取AI对日志的解读和建议</Text>
          )}
        </Card>
      </Spin>
    </div>
  )
}