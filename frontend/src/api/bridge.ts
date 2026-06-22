import { getCredentials, isWebMode, clearCredentials } from './auth'

const isWails = !isWebMode()

const wailsCall = async (fn: string, ...args: any[]) => {
  const app = (window as any).go?.main?.App
  if (!app) throw new Error('后端服务尚未就绪')
  const f = (app as any)[fn]
  if (typeof f !== 'function') throw new Error(`方法 ${fn} 不存在`)
  return await f(...args)
}

const httpCall = async (fn: string, ...args: any[]) => {
  const creds = getCredentials()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (creds) {
    headers['Authorization'] = 'Basic ' + btoa(creds.user + ':' + creds.pass)
  }

  const resp = await fetch('/api/call', {
    method: 'POST',
    headers,
    body: JSON.stringify({ method: fn, args }),
  })

  if (resp.status === 401) {
    clearCredentials()
    window.location.reload()
    throw new Error('未授权')
  }

  if (!resp.ok) {
    const text = await resp.text()
    throw new Error(`HTTP ${resp.status}: ${text}`)
  }

  return await resp.json()
}

const call = isWails ? wailsCall : httpCall

const safeCall = async <T>(fn: string, fallback: T, ...args: any[]): Promise<T> => {
  try {
    const result = await call(fn, ...args)
    return (result === null || result === undefined) ? fallback : result as T
  } catch (e) {
    return fallback
  }
}

export const SearchStock = (keyword: string) =>
  safeCall('SearchStock', [] as any, keyword)

export const GetRealtimeData = (codes: string[]) =>
  safeCall('GetRealtimeData', {} as any, codes)

export const GetKLineData = (code: string, days = 120) =>
  safeCall('GetKLineData', [] as any, code, days)

export const GetTechnicalAnalysis = (code: string) =>
  safeCall('GetTechnicalAnalysis', null, code)

export const GetAIAnalysis = (code: string) =>
  safeCall('GetAIAnalysis', 'AI分析服务不可用，请检查API配置', code)

export const AIChat = (message: string) =>
  safeCall('AIChat', 'AI服务不可用，请检查API配置', message)

export const AIChatWithHistory = (messagesJSON: string, systemPrompt = '') =>
  safeCall('AIChatWithHistory', 'AI服务不可用，请检查API配置', messagesJSON, systemPrompt)

export const GetSelfSelectStocks = () =>
  safeCall('GetSelfSelectStocks', [] as any)

export const GetGroups = () =>
  safeCall('GetGroups', [] as any)

export const AddSelfSelectStock = (code: string, name: string, group: string) =>
  safeCall('AddSelfSelectStock', '添加失败', code, name, group)

export const RemoveSelfSelectStock = (code: string) =>
  safeCall('RemoveSelfSelectStock', '删除失败', code)

export const MoveStockToGroup = (code: string, group: string) =>
  safeCall('MoveStockToGroup', '移动失败', code, group)

export const UpdateStockNotes = (code: string, notes: string) =>
  safeCall('UpdateStockNotes', '更新失败', code, notes)

export const AddGroup = (name: string) =>
  safeCall('AddGroup', '添加分组失败', name)

export const RemoveGroup = (name: string) =>
  safeCall('RemoveGroup', '删除分组失败', name)

export const RenameGroup = (oldName: string, newName: string) =>
  safeCall('RenameGroup', '重命名失败', oldName, newName)

export const GetSettings = () =>
  safeCall('GetSettings', { config: {}, refreshInterval: 2, dataSource: 'auto' })

export const SaveSettings = (settings: any) =>
  safeCall('SaveSettings', '保存失败', settings)

export const RefreshAllSelfSelect = () =>
  safeCall('RefreshAllSelfSelect', [] as any)

export const GetRefreshInterval = () =>
  safeCall('GetRefreshInterval', 2)

export const TestAIConnection = (provider: string, endpoint: string, apiKey: string, model: string) =>
  safeCall('TestAIConnection', '✗ 连接测试: 后端服务未就绪', provider, endpoint, apiKey, model)

export const ListModels = (provider: string, endpoint: string, apiKey: string) =>
  safeCall('ListModels', { models: [], error: '后端服务未就绪' } as any, provider, endpoint, apiKey)

export const GetModelUsages = () =>
  safeCall('GetModelUsages', [] as any)

export const DeleteModelUsage = (provider: string, modelName: string) =>
  safeCall('DeleteModelUsage', '删除失败', provider, modelName)

export const GetTimeSharingData = (code: string) =>
  safeCall('GetTimeSharingData', [] as any, code)

export const GetMoneyFlow = (code: string, days = 30) =>
  safeCall('GetMoneyFlow', [] as any, code, days)

export const GetStockIndustry = (code: string) =>
  safeCall('GetStockIndustry', '', code)

export const GetPosition = (code: string) =>
  safeCall('GetPosition', null, code)

export const GetAllPositions = () =>
  safeCall('GetAllPositions', [] as any)

export const SavePosition = (pos: any) =>
  safeCall('SavePosition', '保存失败', pos)

export const DeletePosition = (code: string) =>
  safeCall('DeletePosition', '删除失败', code)

export const GetMarketSummary = () =>
  safeCall('GetMarketSummary', null as any)

export const GetIndexMoneyFlow = (code: string, days = 10) =>
  safeCall('GetIndexMoneyFlow', [] as any, code, days)

export const SwitchModel = (provider: string, model: string, endpoint: string, apiKey: string) =>
  safeCall('SwitchModel', '', provider, model, endpoint, apiKey)

export const GetSectorMoneyFlow = () =>
  safeCall('GetSectorMoneyFlow', [] as any)

export const GetSectorMoneyFlowByDate = (date: string) =>
  safeCall('GetSectorMoneyFlowByDate', [] as any, date)

export const GetSectorTree = () =>
  safeCall('GetSectorTree', [] as any)

export const GetRealTimePriority = () =>
  safeCall('GetRealTimePriority', [] as string[])

export const SaveRealTimePriority = (priority: string[]) =>
  safeCall('SaveRealTimePriority', '保存失败', priority)

export const GetSourceStats = () =>
  safeCall('GetSourceStats', [] as any)

export const GetLogDates = () =>
  safeCall('GetLogDates', [] as string[])

export const GetLogModules = (date: string) =>
  safeCall('GetLogModules', [] as string[], date)

export const GetLogs = (date: string, module: string) =>
  safeCall('GetLogs', [] as any, date, module)

export const GetLogAIInterpretation = (date: string, module: string) =>
  safeCall('GetLogAIInterpretation', 'AI分析不可用', date, module)

export const GetTTSConfig = () =>
  safeCall('GetTTSConfig', { provider: 'browser', apiKey: '' })

export const SaveTTSConfig = (cfg: any) =>
  safeCall('SaveTTSConfig', '保存失败', cfg)

export const TextToSpeech = (text: string, provider: string) =>
  safeCall('TextToSpeech', '', text, provider)

// SSE-based streaming for web mode
export const AIChatStreamWeb = async (
  messagesJSON: string,
  systemPrompt: string,
  onChunk: (text: string) => void,
  onDone: (text: string) => void,
  onError: (err: string) => void
) => {
  const creds = getCredentials()
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  if (creds) {
    headers['Authorization'] = 'Basic ' + btoa(creds.user + ':' + creds.pass)
  }

  try {
    const resp = await fetch('/api/chat/stream', {
      method: 'POST',
      headers,
      body: JSON.stringify({ messages: messagesJSON, systemPrompt }),
    })

    if (!resp.ok) {
      onError(`HTTP ${resp.status}`)
      return
    }

    const reader = resp.body!.getReader()
    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const data = JSON.parse(line.slice(6))
            if (data.text !== undefined) {
              onChunk(data.text)
            }
            if (data.error) {
              onError(data.error)
              return
            }
          } catch {}
        }
      }
    }

    // Process remaining buffer
    if (buffer.startsWith('data: ')) {
      try {
        const data = JSON.parse(buffer.slice(6))
        if (data.text !== undefined) {
          onDone(data.text)
          return
        }
      } catch {}
    }

    onDone('')
  } catch (e: any) {
    onError(e.message || String(e))
  }
}
