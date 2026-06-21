import { useState, useEffect } from 'react'
import { Tag, Select, Space, Switch, Typography } from 'antd'
import { RobotOutlined, SearchOutlined } from '@ant-design/icons'
import { GetSettings, SwitchModel } from '../api/bridge'

const { Text } = Typography

interface ModelIndicatorProps {
  webSearch?: boolean
  onWebSearchChange?: (v: boolean) => void
  compact?: boolean
}

export default function ModelIndicator({ webSearch, onWebSearchChange, compact }: ModelIndicatorProps) {
  const [currentModel, setCurrentModel] = useState('')
  const [currentProvider, setCurrentProvider] = useState('')
  const [currentEndpoint, setCurrentEndpoint] = useState('')
  const [currentApiKey, setCurrentApiKey] = useState('')
  const [allProviders, setAllProviders] = useState<any[]>([])

  useEffect(() => {
    GetSettings().then((s: any) => {
      if (s?.config) {
        setCurrentModel(s.config.modelName || '')
        setCurrentProvider(s.config.provider || '')
        setCurrentEndpoint(s.config.endpoint || '')
        setCurrentApiKey(s.config.apiKey || '')
      }
      const providers: any[] = []
      const seen = new Set<string>()
      for (const cfg of [s?.config, ...(s?.configs || [])].filter(Boolean)) {
        const key = `${cfg.provider}:${cfg.endpoint}`
        if (seen.has(key)) continue
        seen.add(key)
        const label = `${cfg.provider}${cfg.endpoint ? ` (${cfg.endpoint})` : ''}`
        providers.push({ label, value: key, provider: cfg.provider, endpoint: cfg.endpoint, apiKey: cfg.apiKey, modelName: cfg.modelName })
      }
      setAllProviders(providers)
    })
  }, [])

  const handleModelSwitch = (value: string) => {
    const prov = allProviders.find(p => p.value === value)
    if (!prov) return
    setCurrentProvider(prov.provider)
    setCurrentModel(prov.modelName)
    setCurrentEndpoint(prov.endpoint)
    setCurrentApiKey(prov.apiKey)
    SwitchModel(prov.provider, prov.modelName, prov.endpoint, prov.apiKey)
  }

  if (compact) {
    return (
      <Space size={4}>
        <RobotOutlined style={{ fontSize: 12, color: '#999' }} />
        <Text type="secondary" style={{ fontSize: 11 }}>{currentModel || '未配置'}</Text>
        {onWebSearchChange !== undefined && (
          <>
            <SearchOutlined style={{ fontSize: 12, color: '#999', marginLeft: 8 }} />
            <Switch size="small" checked={!!webSearch} onChange={onWebSearchChange} />
          </>
        )}
      </Space>
    )
  }

  return (
    <Space size={4} style={{ fontSize: 12 }} wrap>
      <RobotOutlined />
      <Text type="secondary">模型:</Text>
      <Select
        size="small"
        style={{ minWidth: 180 }}
        value={allProviders.find(p => p.provider === currentProvider)?.value || ''}
        onChange={handleModelSwitch}
        options={allProviders.map(p => ({ label: p.label, value: p.value }))}
        placeholder="选择模型"
      />
      {currentModel && <Tag style={{ marginLeft: 4, fontSize: 11 }}>{currentModel}</Tag>}
      {onWebSearchChange !== undefined && (
        <Space size={4} style={{ marginLeft: 8 }}>
          <SearchOutlined style={{ color: '#999' }} />
          <Text type="secondary" style={{ fontSize: 12 }}>联网搜索</Text>
          <Switch size="small" checked={!!webSearch} onChange={onWebSearchChange} />
        </Space>
      )}
    </Space>
  )
}