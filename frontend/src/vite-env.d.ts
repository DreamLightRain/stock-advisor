/// <reference types="vite/client" />

declare module '@ant-design/icons'
declare module 'antd/locale/zh_CN'
declare module '*.css'

interface Window {
  go: {
    main: {
      App: {
        SearchStock(keyword: string): Promise<any[]>
        GetRealtimeData(codes: string[]): Promise<Record<string, any>>
        GetKLineData(code: string, days: number): Promise<any[]>
        GetTechnicalAnalysis(code: string): Promise<any>
        GetAIAnalysis(code: string): Promise<string>
        AIChat(message: string): Promise<string>
        GetSelfSelectStocks(): Promise<any[]>
        GetGroups(): Promise<any[]>
        AddSelfSelectStock(code: string, name: string, group: string): Promise<string>
        RemoveSelfSelectStock(code: string): Promise<string>
        MoveStockToGroup(code: string, group: string): Promise<string>
        UpdateStockNotes(code: string, notes: string): Promise<string>
        AddGroup(name: string): Promise<string>
        RemoveGroup(name: string): Promise<string>
        RenameGroup(oldName: string, newName: string): Promise<string>
        GetSettings(): Promise<any>
        SaveSettings(settings: any): Promise<string>
        RefreshAllSelfSelect(): Promise<any[]>
        GetRefreshInterval(): Promise<number>
        TestAIConnection(provider: string, endpoint: string, apiKey: string, model: string): Promise<string>
        ListModels(provider: string, endpoint: string, apiKey: string): Promise<{ models: string[], error: string }>
        GetTimeSharingData(code: string): Promise<any[]>
        GetMoneyFlow(code: string, days: number): Promise<any[]>
        GetPosition(code: string): Promise<any>
        GetAllPositions(): Promise<any[]>
        SavePosition(pos: any): Promise<string>
        DeletePosition(code: string): Promise<string>
      }
    }
  }
}
