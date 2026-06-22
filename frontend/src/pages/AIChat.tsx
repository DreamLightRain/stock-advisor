import { useState, useRef, useEffect, useCallback } from 'react'
import { Input, Button, Card, Typography, Spin, Space, Tag, Row, Col, Switch, Select, message, Modal } from 'antd'
import { SendOutlined, RobotOutlined, BulbOutlined, SearchOutlined, ClearOutlined, EditOutlined, SoundOutlined } from '@ant-design/icons'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { AIChatWithHistory, GetSelfSelectStocks, GetSettings, GetModelUsages, SwitchModel, AIChatStreamWeb, GetTTSConfig, TextToSpeech } from '../api/bridge'
import { isWebMode } from '../api/auth'

const { Text, Title, Paragraph } = Typography
const { TextArea } = Input

interface Message {
  role: 'user' | 'assistant'
  content: string
  timestamp: number
}

const STORAGE_KEY = 'stock_advisor_chat_messages'
const PROMPT_KEY = 'stock_advisor_system_prompt'

const PRESET_PROMPTS: Record<string, string> = {
  '专业分析师': '你是一位专业的A股投资顾问，精通技术分析和基本面分析。请基于客观数据和分析给出建议，并提醒投资风险。回答请用中文，简明扼要，专业客观。使用 ### 标题分段，段落之间单个换行（不要多余空行），让内容紧凑易读。注意：你提供的只是分析建议，不构成投资依据。',
  '价值投资者': '你是一位价值投资专家，擅长巴菲特和格雷厄姆的价值投资理念。分析股票时重点关注：公司基本面、估值水平、ROE、现金流、行业护城河等。给出长期投资建议，避免短线操作。使用 ### 标题分段，段落之间单个换行。',
  '短线交易员': '你是一位短线交易专家，擅长技术分析和市场情绪判断。分析时重点关注：K线形态、成交量变化、资金流向、市场热点、技术指标信号等。给出短线买卖建议和止损位。使用 ### 标题分段，段落之间单个换行。',
}

const suggestions: string[] = [
  '如何分析一只股票的技术面？',
  'MACD金叉和死叉怎么看？',
  '股票成交量放大说明什么？',
  '如何判断支撑位和压力位？',
  '什么是RSI指标？如何使用？',
  '近期市场行情如何分析？',
]

function loadMessages(): Message[] {
  try {
    const data = localStorage.getItem(STORAGE_KEY)
    if (data) return JSON.parse(data)
  } catch { /* ignore */ }
  return []
}

function saveMessages(msgs: Message[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(msgs))
  } catch { /* ignore */ }
}

export default function AIChat() {
  const [messages, setMessages] = useState<Message[]>(() => {
    const saved = loadMessages()
    if (saved.length === 0) {
      return [{
        role: 'assistant',
        content: '你好！我是AI股票分析助手。我可以帮你分析股票、解答投资问题、提供市场观点。请告诉我你想了解什么？\n\n输入 /clear 可以清除对话历史。',
        timestamp: Date.now(),
      }]
    }
    return saved
  })
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [selfStocks, setSelfStocks] = useState<any[]>([])
  const [webSearch, setWebSearch] = useState(false)
  const [currentModel, setCurrentModel] = useState('')
  const [currentProvider, setCurrentProvider] = useState('')
  const [currentEndpoint, setCurrentEndpoint] = useState('')
  const [currentApiKey, setCurrentApiKey] = useState('')
  const [providerModels, setProviderModels] = useState<Record<string, string[]>>({})
  const [allProviders, setAllProviders] = useState<any[]>([])
  const [promptModalOpen, setPromptModalOpen] = useState(false)
  const [systemPrompt, setSystemPrompt] = useState(() => localStorage.getItem(PROMPT_KEY) || PRESET_PROMPTS['专业分析师'])
  const [selectedPreset, setSelectedPreset] = useState('专业分析师')
  const [ttsProvider, setTtsProvider] = useState('edge')
  const [ttsVoice, setTtsVoice] = useState('zh-CN-XiaoxiaoNeural')
  const [speakingIndex, setSpeakingIndex] = useState<number | null>(null)
  const ttsAudioRef = useRef<HTMLAudioElement | null>(null)
  const [streamingContent, setStreamingContent] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    GetTTSConfig().then((cfg: any) => {
      if (cfg) {
        setTtsProvider(cfg.provider || 'edge')
        setTtsVoice(cfg.voice || 'zh-CN-XiaoxiaoNeural')
      }
    })
  }, [])

  const playTTS = async (text: string, idx: number) => {
    // Stop if already playing this message
    if (speakingIndex === idx) {
      if (ttsAudioRef.current) { ttsAudioRef.current.pause(); ttsAudioRef.current = null }
      if (ttsProvider === 'browser') window.speechSynthesis.cancel()
      setSpeakingIndex(null)
      return
    }

    // Stop any current playback
    if (ttsAudioRef.current) { ttsAudioRef.current.pause(); ttsAudioRef.current = null }
    if (ttsProvider === 'browser') window.speechSynthesis.cancel()

    const cleanText = text.replace(/[#*`\[\]]/g, '')

    if (ttsProvider === 'browser') {
      const utterance = new SpeechSynthesisUtterance(cleanText)
      utterance.lang = 'zh-CN'
      utterance.rate = 1.1
      utterance.onend = () => setSpeakingIndex(null)
      utterance.onerror = () => setSpeakingIndex(null)
      setSpeakingIndex(idx)
      window.speechSynthesis.speak(utterance)
      return
    }

    // Edge TTS (and others) via backend
    setSpeakingIndex(idx)
    try {
      const b64 = await TextToSpeech(cleanText, ttsProvider, ttsVoice)
      if (!b64) {
        message.warning('语音合成失败，请检查设置')
        setSpeakingIndex(null)
        return
      }
      const audio = new Audio('data:audio/mp3;base64,' + b64)
      ttsAudioRef.current = audio
      audio.onended = () => { setSpeakingIndex(null); ttsAudioRef.current = null }
      audio.onerror = () => { setSpeakingIndex(null); ttsAudioRef.current = null }
      audio.play()
    } catch {
      setSpeakingIndex(null)
    }
  }

  // Load settings, model usages, and self stocks on mount
  useEffect(() => {
    GetSelfSelectStocks().then(setSelfStocks)

    const loadProviders = async () => {
      // Load current settings for refreshInterval/dataSource defaults
      const s = await GetSettings()

      // Load ALL configured models from usages
      const usages = await GetModelUsages()
      const providers: any[] = []
      const models: Record<string, string[]> = {}
      const seen = new Set<string>()

      // Add current config first (may not be in usages if never saved)
      const sConfig = (s as any)?.config
      if (sConfig?.provider) {
        const key = `${sConfig.provider}:${sConfig.modelName}:${sConfig.endpoint}`
        if (!seen.has(key)) {
          seen.add(key)
          const label = `${sConfig.provider} / ${sConfig.modelName}${sConfig.endpoint ? ` (${sConfig.endpoint})` : ''}`
          if (!models[sConfig.provider]) models[sConfig.provider] = []
          providers.push({ label, value: key, provider: sConfig.provider, endpoint: sConfig.endpoint, apiKey: sConfig.apiKey, modelName: sConfig.modelName })
          setCurrentModel(sConfig.modelName || '')
          setCurrentProvider(sConfig.provider || '')
          setCurrentEndpoint(sConfig.endpoint || '')
          setCurrentApiKey(sConfig.apiKey || '')
        }
      }

      // Add all models from ModelUsages
      for (const u of (usages || []) as any[]) {
        const prov = u.provider || ''
        const mName = u.modelName || ''
        const uKey = u.apiKey || ''
        const uEp = u.endpoint || ''
        // Deduplicate: skip if already seen (same provider+model+endpoint)
        const key = `${prov}:${mName}:${uEp}`
        if (seen.has(key)) continue
        seen.add(key)
        const label = `${prov} / ${mName}${uEp ? ` (${uEp})` : ''}`
        if (!models[prov]) models[prov] = []
        // Use stored apiKey if available, otherwise fall back to current config's apiKey
        const effectiveKey = uKey || (prov === sConfig?.provider ? (sConfig.apiKey || '') : '')
        const effectiveEp = uEp || (prov === sConfig?.provider ? (sConfig.endpoint || '') : '')
        providers.push({ label, value: key, provider: prov, endpoint: effectiveEp, apiKey: effectiveKey, modelName: mName })
      }

      setAllProviders(providers)
      setProviderModels(models)
    }

    loadProviders()
  }, [])

  // Scroll to bottom
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, streamingContent])

  // Save messages on change
  useEffect(() => {
    saveMessages(messages)
  }, [messages])

  // Clean up Wails event listeners on unmount
  useEffect(() => {
    return () => {
      if (!isWebMode()) {
        // Remove any leftover Wails event listeners
        try { (window as any).runtime?.EventsOff?.('ai:stream:chunk') } catch {}
        try { (window as any).runtime?.EventsOff?.('ai:stream:done') } catch {}
        try { (window as any).runtime?.EventsOff?.('ai:stream:error') } catch {}
      }
    }
  }, [])

  const sendMessage = useCallback(async (text: string, isQuickAction = false) => {
    const msg = text.trim()
    if (!msg || loading || isStreaming) return

    // Handle /clear command
    if (msg === '/clear') {
      const reset: Message[] = [{
        role: 'assistant',
        content: '对话历史已清除，开始新的对话吧！',
        timestamp: Date.now(),
      }]
      setMessages(reset)
      setInput('')
      message.success('上下文已清理')
      return
    }

    const userMsg: Message = { role: 'user', content: msg, timestamp: Date.now() }
    const newMessages = [...messages, userMsg]
    setMessages(newMessages)
    if (!isQuickAction) setInput('')
    setLoading(true)
    setIsStreaming(true)
    setStreamingContent('')

    // Build messages JSON for backend
    const backendMessages = newMessages.map(m => ({ role: m.role, content: m.content }))
    const prefix = webSearch ? '[联网搜索] ' : ''
    backendMessages[backendMessages.length - 1].content = prefix + msg

    const fullContentRef = { current: '' }

    const onChunk = (chunk: string) => {
      fullContentRef.current += chunk
      setStreamingContent(fullContentRef.current)
    }

    const onDone = (finalText: string) => {
      const text = finalText || fullContentRef.current
      const assistantMsg: Message = { role: 'assistant', content: text, timestamp: Date.now() }
      setMessages(prev => [...prev, assistantMsg])
      setStreamingContent('')
      setIsStreaming(false)
      setLoading(false)
    }

    const onError = (err: string) => {
      setIsStreaming(false)
      setLoading(false)
      message.error(err || 'AI响应失败')
    }

    const messagesJSON = JSON.stringify(backendMessages)

    try {
      if (isWebMode()) {
        // Web mode: use SSE streaming
        await AIChatStreamWeb(messagesJSON, systemPrompt, onChunk, onDone, onError)
      } else {
        // Wails mode: use Wails streaming events
        const app = (window as any).go?.main?.App
        if (!app?.AIChatStream) {
          // Fallback: non-streaming
          const reply = await AIChatWithHistory(messagesJSON, systemPrompt)
          if (reply) {
            onChunk(reply)
            onDone(reply)
          } else {
            onError('AI返回为空')
          }
          return
        }

        const removeChunk = (window as any).runtime?.EventsOn?.('ai:stream:chunk', (chunk: string) => {
          onChunk(chunk)
        })
        const removeDone = (window as any).runtime?.EventsOn?.('ai:stream:done', (fullText: string) => {
          removeChunk?.()
          removeDone?.()
          removeError?.()
          onDone(fullText || fullContentRef.current)
        })
        const removeError = (window as any).runtime?.EventsOn?.('ai:stream:error', (err: string) => {
          removeChunk?.()
          removeDone?.()
          removeError?.()
          onError(err)
        })

        app.AIChatStream(messagesJSON, systemPrompt)
      }
    } catch (err: any) {
      // Wails fallback: try non-streaming
      try {
        const reply = await AIChatWithHistory(messagesJSON, systemPrompt)
        if (reply && !reply.startsWith('AI回复出错') && !reply.startsWith('AI客户端')) {
          onChunk(reply)
          onDone(reply)
        } else {
          onError(reply || err?.message || 'AI响应失败')
        }
      } catch (e2: any) {
        onError(e2?.message || 'AI响应失败')
      }
    }
  }, [messages, loading, isStreaming, webSearch, systemPrompt])

  const handleSend = () => {
    sendMessage(input)
  }

  const handleClear = () => {
    const reset: Message[] = [{
      role: 'assistant',
      content: '对话历史已清除，开始新的对话吧！',
      timestamp: Date.now(),
    }]
    setMessages(reset)
    message.success('上下文已清理')
  }

  const analyzeSelfStock = async (code: string, name: string) => {
    const text = `请分析 ${name} (${code}) 的技术面`
    sendMessage(text, true)
  }

  const handleModelSwitch = (value: string) => {
    const prov = allProviders.find(p => p.value === value)
    if (!prov) return
    if (!prov.apiKey) {
      message.warning(`模型 ${prov.modelName} 未配置API密钥，请在设置中配置后再使用`)
    }
    setCurrentProvider(prov.provider)
    setCurrentModel(prov.modelName)
    setCurrentEndpoint(prov.endpoint)
    setCurrentApiKey(prov.apiKey)
    SwitchModel(prov.provider, prov.modelName, prov.endpoint, prov.apiKey).then((res: any) => {
      if (res === 'ok') {
        message.success(`已切换至 ${prov.label}`)
      }
    })
  }

  const handlePresetChange = (preset: string) => {
    setSelectedPreset(preset)
    if (preset !== '自定义') {
      setSystemPrompt(PRESET_PROMPTS[preset])
    }
  }

  const savePrompt = () => {
    localStorage.setItem(PROMPT_KEY, systemPrompt)
    setPromptModalOpen(false)
    message.success('提示词已保存')
  }

  const totalContextChars = messages.reduce((s, m) => s + m.content.length, 0)

  // Render message content with markdown + source attribution
  const renderContent = (content: string) => {
    const parts = content.split(/(数据来源:[\s\S]*)/)
    if (parts.length > 1) {
      return (
        <>
          <div className="ai-message" style={{ fontSize: 14 }}>
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{parts[0]}</ReactMarkdown>
          </div>
          {parts[1] && (
            <div style={{ marginTop: 8, padding: '6px 10px', background: '#f5f5f5', borderRadius: 6, fontSize: 11, color: '#999' }}>
              {parts[1]}
            </div>
          )}
        </>
      )
    }
    return (
      <div className="ai-message" style={{ fontSize: 14 }}>
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 140px)' }}>
      {/* Header */}
      <div style={{ marginBottom: 12 }}>
        <Row justify="space-between" align="middle" style={{ marginBottom: 8 }}>
          <Col>
            <Title level={4} style={{ margin: 0 }}>
              <RobotOutlined /> AI智能分析
            </Title>
          </Col>
          <Col>
            <Space size="middle">
              <Space size={4}>
                <SearchOutlined style={{ color: '#999' }} />
                <Text type="secondary" style={{ fontSize: 12 }}>联网搜索</Text>
                <Switch size="small" checked={webSearch} onChange={setWebSearch} />
              </Space>
              <Button size="small" icon={<EditOutlined />} onClick={() => setPromptModalOpen(true)}>
                提示词
              </Button>
              <Button size="small" icon={<ClearOutlined />} onClick={handleClear}>
                清除
              </Button>
            </Space>
          </Col>
        </Row>
        <Row align="middle" gutter={8}>
          <Col flex="auto">
            <Space size={4} style={{ fontSize: 12 }} wrap>
              <Text type="secondary">模型:</Text>
              <Select
                size="small"
                style={{ minWidth: 240 }}
                value={allProviders.find(p => p.provider === currentProvider && p.modelName === currentModel)?.value || ''}
                onChange={handleModelSwitch}
                options={allProviders.map(p => ({ label: p.label, value: p.value }))}
                placeholder="选择模型"
              />
              <Text type="secondary" style={{ fontSize: 11 }}>
                上下文: {(totalContextChars / 1000).toFixed(0)}K
              </Text>
            </Space>
          </Col>
        </Row>
      </div>

      {/* Quick stock analysis */}
      {selfStocks.length > 0 && (
        <div style={{ marginBottom: 12 }}>
          <Text type="secondary" style={{ marginRight: 8 }}>快速分析自选股:</Text>
          <Space wrap>
            {selfStocks.map((s: any) => (
              <Tag
                key={s.code}
                color="blue"
                style={{ cursor: 'pointer' }}
                onClick={() => analyzeSelfStock(s.code, s.name)}
              >
                {s.name}
              </Tag>
            ))}
          </Space>
        </div>
      )}

      {/* Messages */}
      <Card
        style={{ flex: 1, overflowY: 'auto', marginBottom: 12, background: '#fafafa' }}
        styles={{ body: { padding: 12 } }}
      >
        {messages.map((msg, i) => (
          <div
            key={i}
            style={{
              display: 'flex',
              marginBottom: 16,
              justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
            }}
          >
            <div
              style={{
                maxWidth: '80%',
                padding: '10px 14px',
                borderRadius: 12,
                background: msg.role === 'user' ? '#1677ff' : '#fff',
                color: msg.role === 'user' ? '#fff' : '#333',
                boxShadow: '0 1px 4px rgba(0,0,0,0.1)',
                border: msg.role === 'user' ? 'none' : '1px solid #f0f0f0',
              }}
            >
              <div style={{ fontSize: 12, marginBottom: 4, opacity: 0.7 }}>
                {msg.role === 'user' ? '你' : 'AI助手'}
                {msg.role === 'assistant' && webSearch && (
                  <Tag style={{ marginLeft: 6, fontSize: 10, lineHeight: '14px' }} color="green">联网</Tag>
                )}
              </div>
              {renderContent(msg.content)}
              {msg.role === 'assistant' && (
                <Button
                  type="text"
                  size="small"
                  icon={<SoundOutlined />}
                  onClick={() => playTTS(msg.content, i)}
                  style={{
                    marginTop: 4, fontSize: 12, color: speakingIndex === i ? '#1677ff' : '#999',
                    padding: '2px 6px', height: 'auto',
                  }}
                >
                  {speakingIndex === i ? '停止' : '朗读'}
                </Button>
              )}
            </div>
          </div>
        ))}

        {/* Streaming message */}
        {isStreaming && streamingContent && (
          <div style={{ display: 'flex', marginBottom: 16, justifyContent: 'flex-start' }}>
            <div style={{
              maxWidth: '80%', padding: '10px 14px', borderRadius: 12,
              background: '#fff', color: '#333',
              boxShadow: '0 1px 4px rgba(0,0,0,0.1)',
              border: '1px solid #f0f0f0',
            }}>
              <div style={{ fontSize: 12, marginBottom: 4, opacity: 0.7 }}>AI助手</div>
              <div className="ai-message" style={{ fontSize: 14 }}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{streamingContent}</ReactMarkdown>
              </div>
            </div>
          </div>
        )}

        {loading && !isStreaming && (
          <div style={{ textAlign: 'center', padding: 20 }}>
            <Spin tip="AI思考中..." />
          </div>
        )}
        <div ref={messagesEndRef} />
      </Card>

      {/* Suggestions */}
      <div style={{ marginBottom: 8 }}>
        <Text type="secondary" style={{ fontSize: 12 }}>快速提问:</Text>
        <Space wrap size={4} style={{ marginTop: 4 }}>
          {suggestions.map((s) => (
            <Tag
              key={s}
              style={{ cursor: 'pointer', fontSize: 12 }}
              onClick={() => setInput(s)}
            >
              <BulbOutlined /> {s}
            </Tag>
          ))}
        </Space>
      </div>

      {/* Input */}
      <div style={{ display: 'flex', gap: 8 }}>
        <TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="输入你想了解的股票问题... (/clear 清除上下文)"
          autoSize={{ minRows: 2, maxRows: 4 }}
          onPressEnter={(e: any) => {
            if (!e.shiftKey) {
              e.preventDefault()
              handleSend()
            }
          }}
          disabled={loading || isStreaming}
        />
        <Button
          type="primary"
          icon={<SendOutlined />}
          onClick={handleSend}
          loading={loading || isStreaming}
          style={{ height: 'auto', minHeight: 52 }}
        >
          发送
        </Button>
      </div>

      {/* Prompt editor modal */}
      <Modal
        title={<><EditOutlined /> AI提示词设置</>}
        open={promptModalOpen}
        onCancel={() => setPromptModalOpen(false)}
        onOk={savePrompt}
        okText="保存"
        width={600}
      >
        <div style={{ marginBottom: 12 }}>
          <Text strong>预设提示词</Text>
          <div style={{ marginTop: 8 }}>
            <Space wrap>
              {Object.keys(PRESET_PROMPTS).map((name) => (
                <Tag
                  key={name}
                  color={selectedPreset === name ? 'blue' : 'default'}
                  style={{ cursor: 'pointer' }}
                  onClick={() => handlePresetChange(name)}
                >
                  {name}
                </Tag>
              ))}
              <Tag
                color={selectedPreset === '自定义' ? 'blue' : 'default'}
                style={{ cursor: 'pointer' }}
                onClick={() => { setSelectedPreset('自定义'); setSystemPrompt('') }}
              >
                自定义
              </Tag>
            </Space>
          </div>
        </div>
        <TextArea
          value={systemPrompt}
          onChange={(e) => { setSystemPrompt(e.target.value); setSelectedPreset('自定义') }}
          rows={8}
          placeholder="输入自定义系统提示词..."
        />
        <div style={{ marginTop: 8 }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            提示词将影响AI的回答风格和内容。修改后需要发送新消息才会生效。
          </Text>
        </div>
      </Modal>
    </div>
  )
}
