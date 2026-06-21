import { useState, useEffect } from 'react'
import {
  Card, Form, Input, Select, InputNumber, Button, Typography, message, Divider, Space, Tag, Alert, Switch, Table, Modal, Descriptions
} from 'antd'
import {
  SaveOutlined, ApiOutlined, RobotOutlined, ClockCircleOutlined, CheckCircleOutlined, CloseCircleOutlined, DeleteOutlined, BarChartOutlined, EyeOutlined, ArrowUpOutlined, ArrowDownOutlined, ReloadOutlined
} from '@ant-design/icons'
import { GetSettings, SaveSettings, TestAIConnection, ListModels, GetModelUsages, DeleteModelUsage, GetRealTimePriority, SaveRealTimePriority, GetSourceStats } from '../api/bridge'

const { Title, Text } = Typography

const aiProviders = [
  { value: 'openai', label: 'OpenAI API', desc: '兼容OpenAI协议的服务 (如ChatGPT, DeepSeek, 火山引擎等)' },
  { value: 'ollama', label: 'Ollama 本地模型', desc: '本地部署的Ollama服务 (默认 http://localhost:11434)' },
  { value: 'deepseek', label: 'DeepSeek API', desc: 'DeepSeek官方API (https://api.deepseek.com)' },
  { value: 'volcano', label: '火山引擎', desc: '火山引擎方舟大模型服务' },
  { value: 'opencode', label: 'OpenCode Zen', desc: 'OpenCode Zen免费模型 (需在 opencode.ai 注册获取API Key)' },
]

const providerEndpoints: Record<string, string> = {
  openai: 'https://api.openai.com',
  ollama: 'http://localhost:11434',
  deepseek: 'https://api.deepseek.com',
  volcano: 'https://ark.cn-beijing.volces.com',
  opencode: 'https://opencode.ai/zen',
}

export default function Settings() {
  const [form] = Form.useForm()
  const currentModel = Form.useWatch('modelName', form)
  const [provider, setProvider] = useState('openai')
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<'idle' | 'ok' | 'fail'>('idle')
  const [testMsg, setTestMsg] = useState('')
  const [loadingModels, setLoadingModels] = useState(false)
  const [models, setModels] = useState<string[]>([])
  const [modelError, setModelError] = useState('')
  const [modelUsages, setModelUsages] = useState<any[]>([])
  const [detailModal, setDetailModal] = useState<{ open: boolean; record: any }>({ open: false, record: null })

  // Data source priority
  const sourceLabels: Record<string, string> = { sina: '新浪财经', tencent: '腾讯财经', tdx: '通达信 TCP' }
  const [priority, setPriority] = useState<string[]>([])
  const [sourceStats, setSourceStats] = useState<any[]>([])
  const [priorityDirty, setPriorityDirty] = useState(false)

  const reloadUsages = () => GetModelUsages().then(setModelUsages)
  const reloadPriority = () => {
    GetRealTimePriority().then((p: string[]) => setPriority(p.length ? p : ['sina', 'tencent', 'tdx']))
    GetSourceStats().then(setSourceStats)
  }

  useEffect(() => {
    GetSettings().then((settings) => {
      if (settings?.config) {
        const vals = {
          provider: settings.config.provider || 'openai',
          endpoint: settings.config.endpoint || '',
          apiKey: settings.config.apiKey || '',
          modelName: settings.config.modelName || '',
          maxTokens: settings.config.maxTokens ?? 100000,
          timeout: settings.config.timeout ?? 5,
          refreshInterval: settings.refreshInterval ?? 2,
          dataSource: settings.dataSource || 'auto',
        }
        form.setFieldsValue(vals)
        setProvider(settings.config.provider || 'openai')
      }
    })
    reloadUsages()
    reloadPriority()
  }, [form])

  const movePriority = (idx: number, dir: -1 | 1) => {
    const newP = [...priority]
    const target = idx + dir
    if (target < 0 || target >= newP.length) return
    ;[newP[idx], newP[target]] = [newP[target], newP[idx]]
    setPriority(newP)
    setPriorityDirty(true)
  }

  const handleTest = async () => {
    setTesting(true)
    setTestResult('idle')
    setTestMsg('')
    try {
      const values = form.getFieldsValue()
      const res = await TestAIConnection(values.provider, values.endpoint, values.apiKey, values.modelName)
      if (res === 'ok') {
        setTestResult('ok')
        setTestMsg('✓ 连接成功！API工作正常')
        message.success('连接测试通过')
        // Auto-save on success
        const settings = {
          config: {
            provider: values.provider,
            endpoint: values.endpoint,
            apiKey: values.apiKey,
            modelName: values.modelName,
            maxTokens: values.maxTokens ?? 100000,
            timeout: values.timeout ?? 5,
          },
          refreshInterval: values.refreshInterval ?? 2,
          dataSource: values.dataSource || 'auto',
        }
        const saveRes = await SaveSettings(settings)
        if (saveRes === 'ok') {
          message.success('配置已自动保存')
        }
        reloadUsages()
      } else {
        setTestResult('fail')
        setTestMsg(res)
      }
    } catch (e: any) {
      setTestResult('fail')
      setTestMsg(`✗ 发生异常: ${String(e)}`)
    }
    setTesting(false)
  }

  const handleListModels = async () => {
    setLoadingModels(true)
    setModels([])
    setModelError('')
    try {
      const values = form.getFieldsValue()
      const result = await ListModels(values.provider, values.endpoint, values.apiKey)
      if (result.error) {
        setModelError(result.error)
        message.error(result.error)
      } else if (result.models && result.models.length > 0) {
        setModels(result.models)
        message.success(`成功获取 ${result.models.length} 个模型`)
      } else {
        setModelError('未返回任何模型，请检查服务状态')
      }
    } catch (e: any) {
      setModelError(`请求异常: ${String(e)}`)
    }
    setLoadingModels(false)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      const values = await form.validateFields()
      const settings = {
        config: {
          provider: values.provider,
          endpoint: values.endpoint,
          apiKey: values.apiKey,
          modelName: values.modelName,
          maxTokens: values.maxTokens ?? 100000,
          timeout: values.timeout ?? 5,
        },
        refreshInterval: values.refreshInterval ?? 2,
        dataSource: values.dataSource || 'auto',
      }
      const res = await SaveSettings(settings)
      if (res === 'ok') {
        if (priorityDirty) {
          await SaveRealTimePriority(priority)
          setPriorityDirty(false)
        }
        message.success('设置已保存')
        reloadUsages()
      } else {
        message.error(res)
      }
    } catch {
      message.error('保存失败，请检查表单')
    }
    setSaving(false)
  }

  const handleProviderChange = (val: string) => {
    setProvider(val)
    setModels([])
    setModelError('')
    const ep = providerEndpoints[val] || ''
    const currentEp = form.getFieldValue('endpoint')
    if (!currentEp) {
      form.setFieldsValue({ endpoint: ep })
    }
  }

  const handleDeleteUsage = async (record: any) => {
    const res = await DeleteModelUsage(record.provider, record.modelName)
    if (res === 'ok') {
      message.success('已删除')
      reloadUsages()
    } else {
      message.error(res)
    }
  }

  const handleSelectUsage = (record: any) => {
    form.setFieldsValue({
      provider: record.provider,
      endpoint: record.endpoint || '',
      apiKey: record.apiKey || '',
      modelName: record.modelName,
    })
    setProvider(record.provider)
    setDetailModal({ open: true, record })
    message.info(`已选择模型: ${record.modelName}`)
  }

  const usagesColumns = [
    { title: '服务商', dataIndex: 'provider', key: 'provider', render: (v: string) => <Tag>{v}</Tag> },
    { title: '模型', dataIndex: 'modelName', key: 'modelName' },
    { title: '端点', dataIndex: 'endpoint', key: 'endpoint', render: (v: string) => v ? <Text copyable={{ text: v }} style={{ fontSize: 12 }}>{v}</Text> : '-', responsive: ['md' as any] },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => (
      <Tag color={v === 'available' ? 'green' : v === 'error' ? 'red' : 'default'}>
        {v === 'available' ? '可用' : v === 'error' ? '异常' : '未知'}
      </Tag>
    )},
    { title: '输入Token', dataIndex: 'inputTokens', key: 'inputTokens', render: (v: number) => v ? v.toLocaleString() : '-' },
    { title: '输出Token', dataIndex: 'outputTokens', key: 'outputTokens', render: (v: number) => v ? v.toLocaleString() : '-' },
    { title: '请求次数', dataIndex: 'totalRequests', key: 'totalRequests', render: (v: number) => v || 0 },
    { title: '最近测试', dataIndex: 'lastTest', key: 'lastTest', render: (v: string) => v || '-' },
    { title: '操作', key: 'action', render: (_: any, record: any) => (
      <Button type="text" danger icon={<DeleteOutlined />} onClick={() => handleDeleteUsage(record)} />
    )},
  ]

  return (
    <div style={{ maxWidth: 800 }}>
      <Title level={4}><ApiOutlined /> 系统设置</Title>

      <Form
        form={form}
        layout="vertical"
        onFinish={handleSave}
        initialValues={{
          provider: 'openai', refreshInterval: 2,
          maxTokens: 100000, timeout: 5,
          dataSource: 'auto',
        }}
      >
        {/* Configured Models */}
        <Card title={<><RobotOutlined /> 已配置供应商/模型</>} style={{ marginBottom: 16 }}>
          {modelUsages.length > 0 ? (
            <Table
              dataSource={modelUsages}
              columns={usagesColumns}
              rowKey={(r: any) => r.provider + r.modelName}
              size="small"
              pagination={false}
              onRow={(record) => ({
                style: { cursor: 'pointer' },
                onClick: () => handleSelectUsage(record),
              })}
            />
          ) : (
            <Text type="secondary">暂无已配置的模型，请先配置AI服务</Text>
          )}
        </Card>

        {/* AI Configuration */}
        <Card id="ai-config-section" title={<><RobotOutlined /> 配置AI模型</>} style={{ marginBottom: 16 }}>
          <Alert
            message="AI功能需要自行配置。支持OpenAI协议兼容的服务和本地Ollama。配置后可点击「测试连接」验证，通过后自动保存。"
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />

          <Form.Item label="AI服务商" name="provider">
            <Select onChange={handleProviderChange}>
              {aiProviders.map(p => (
                <Select.Option key={p.value} value={p.value}>{p.label}</Select.Option>
              ))}
            </Select>
          </Form.Item>

          {(() => {
            const cfg = aiProviders.find(p => p.value === provider)
            return cfg ? <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>{cfg.desc}</Text> : null
          })()}

          <Form.Item label="API端点地址" name="endpoint"
            extra={provider === 'ollama' ? 'Ollama v0.3+ 支持OpenAI兼容接口' : ''}
          >
            <Input placeholder={providerEndpoints[provider] || 'https://api.openai.com'} />
          </Form.Item>

          {provider !== 'ollama' && (
            <Form.Item label="API密钥" name="apiKey">
              <Input.Password placeholder="输入你的API Key" visibilityToggle />
            </Form.Item>
          )}

          <Form.Item label="当前模型" name="modelName"
            extra="从下方模型列表点击选择，或手动输入"
          >
            <Select
              mode="tags"
              maxCount={1}
              placeholder="选择或输入模型名称"
              onChange={(val: any) => {
                if (Array.isArray(val) && val.length > 0) {
                  form.setFieldsValue({ modelName: val[val.length - 1] })
                }
              }}
              value={currentModel ? [currentModel] : []}
            >
              {models.map(m => (
                <Select.Option key={m} value={m}>{m}</Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Space>
            <Form.Item label="最大Token数" name="maxTokens">
              <InputNumber min={256} max={1048576} step={1024} style={{ width: 160 }} />
            </Form.Item>
            <Form.Item label="超时(秒)" name="timeout">
              <InputNumber min={1} max={120} style={{ width: 100 }} />
            </Form.Item>
          </Space>

          <Divider />

          <Space direction="vertical" style={{ width: '100%' }}>
            <Space wrap>
              <Button
                type="primary"
                icon={<CheckCircleOutlined />}
                onClick={handleTest}
                loading={testing}
              >
                测试并添加模型
              </Button>
              <Button
                icon={<RobotOutlined />}
                onClick={handleListModels}
                loading={loadingModels}
              >
                获取模型列表
              </Button>
            </Space>

            {testResult !== 'idle' && (
              <Alert
                type={testResult === 'ok' ? 'success' : 'error'}
                message={testMsg}
                showIcon
                closable
                onClose={() => setTestResult('idle')}
                style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}
              />
            )}

            {modelError && (
              <Alert
                type="error"
                message={<Text style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{modelError}</Text>}
                showIcon
                closable
                onClose={() => setModelError('')}
              />
            )}

            {models.length > 0 && (
              <div>
                <div style={{ marginBottom: 8 }}>
                  <Text type="secondary" style={{ display: 'block', marginBottom: 4 }}>
                    可用模型 (点击标签选择):
                  </Text>
                  <Space wrap>
                    {models.map(m => (
                      <Tag
                        key={m}
                        color={currentModel === m ? 'blue' : 'default'}
                        style={{ cursor: 'pointer' }}
                        onClick={() => form.setFieldsValue({ modelName: m })}
                      >
                        {m}
                      </Tag>
                    ))}
                  </Space>
                </div>
              </div>
            )}
          </Space>
        </Card>

        <Card title={<><ClockCircleOutlined /> 刷新设置</>} style={{ marginBottom: 16 }}>
          <Form.Item
            label="自动刷新间隔(秒)"
            name="refreshInterval"
            help="设为0关闭自动刷新，最低0.01秒"
          >
            <InputNumber
              min={0}
              max={300}
              step={0.01}
              style={{ width: 160 }}
              precision={2}
            />
          </Form.Item>
        </Card>

        <Card title={<><ApiOutlined /> 数据源设置</>} style={{ marginBottom: 16 }}>
          <Form.Item
            label="资金流向数据源"
            name="dataSource"
            help="选择资金流向历史数据的来源。自动模式优先使用 push2delay，失败时回退到数据中心。"
          >
            <Select style={{ width: 240 }}>
              <Select.Option value="auto">自动 (推荐)</Select.Option>
              <Select.Option value="eastmoney">东方财富 (push2delay)</Select.Option>
              <Select.Option value="datacenter">东方财富数据中心 (datacenter)</Select.Option>
              <Select.Option value="sina">新浪财经</Select.Option>
              <Select.Option value="tdx">通达信 TCP</Select.Option>
            </Select>
          </Form.Item>

          <Divider />

          <div style={{ marginBottom: 8 }}><Text strong>实时行情数据源优先级</Text></div>
          <Text type="secondary" style={{ display: 'block', marginBottom: 12, fontSize: 12 }}>
            拖拽调整优先级顺序。自动模式下按此顺序尝试获取实时行情，第一个成功即返回。
          </Text>
          <Space direction="vertical" style={{ width: '100%' }}>
            {priority.map((name, idx) => (
              <div key={name} style={{
                display: 'flex', alignItems: 'center', justifyContent: 'space-between',
                padding: '6px 12px', border: '1px solid #d9d9d9', borderRadius: 6, background: '#fafafa',
              }}>
                <Space>
                  <Tag color="blue" style={{ minWidth: 20, textAlign: 'center' }}>{idx + 1}</Tag>
                  <Text>{sourceLabels[name] || name}</Text>
                  {(() => {
                    const stat = sourceStats.find((s: any) => s.name === sourceLabels[name])
                    if (!stat) return null
                    const total = stat.success + stat.failures
                    const rate = total > 0 ? (stat.success / total * 100).toFixed(0) : '-'
                    return (
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        成功率: {rate}% ({stat.success}/{total})
                      </Text>
                    )
                  })()}
                </Space>
                <Space>
                  <Button size="small" icon={<ArrowUpOutlined />} disabled={idx === 0} onClick={() => movePriority(idx, -1)} />
                  <Button size="small" icon={<ArrowDownOutlined />} disabled={idx === priority.length - 1} onClick={() => movePriority(idx, 1)} />
                </Space>
              </div>
            ))}
          </Space>
          {priorityDirty && <Alert type="info" message="优先级已修改，保存设置后生效" showIcon style={{ marginTop: 8 }} />}

          <Divider />

          <div style={{ marginBottom: 8 }}><Text strong>数据源运行统计</Text></div>
          <Table
            dataSource={sourceStats}
            columns={[
              { title: '数据源', dataIndex: 'name', key: 'name' },
              { title: '成功次数', dataIndex: 'success', key: 'success' },
              { title: '失败次数', dataIndex: 'failures', key: 'failures' },
              { title: '成功率', key: 'rate', render: (_: any, r: any) => {
                const total = r.success + r.failures
                return total > 0 ? `${(r.success / total * 100).toFixed(1)}%` : '-'
              }},
              { title: '最后错误', dataIndex: 'lastError', key: 'lastError', render: (v: string) => v || '-', ellipsis: true },
            ]}
            rowKey="name"
            size="small"
            pagination={false}
          />
        </Card>

        <Form.Item>
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving} size="large">
            保存设置
          </Button>
        </Form.Item>
      </Form>

      {/* Detail Modal */}
      <Modal
        title={detailModal.record?.modelName || '模型详情'}
        open={detailModal.open}
        onCancel={() => setDetailModal({ open: false, record: null })}
        footer={<Button onClick={() => setDetailModal({ open: false, record: null })}>关闭</Button>}
      >
        {detailModal.record && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="服务商">{detailModal.record.provider}</Descriptions.Item>
            <Descriptions.Item label="模型名称">{detailModal.record.modelName}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={detailModal.record.status === 'available' ? 'green' : 'red'}>
                {detailModal.record.status === 'available' ? '可用' : '异常'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="端点地址">{detailModal.record.endpoint || '-'}</Descriptions.Item>
            <Descriptions.Item label="API密钥">{detailModal.record.apiKey ? '***已配置***' : '未配置'}</Descriptions.Item>
            <Descriptions.Item label="输入Token">{detailModal.record.inputTokens?.toLocaleString() || '-'}</Descriptions.Item>
            <Descriptions.Item label="输出Token">{detailModal.record.outputTokens?.toLocaleString() || '-'}</Descriptions.Item>
            <Descriptions.Item label="请求次数">{detailModal.record.totalRequests || 0}</Descriptions.Item>
            <Descriptions.Item label="最后测试时间">{detailModal.record.lastTest || '-'}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}