# Stock Advisor

A-share (沪深A股) stock market analysis desktop application built with [Wails](https://wails.io/) v2 + React 18 + TypeScript + Ant Design 5 + @ant-design/charts.

## Features

- **K-line charts** with historical money flow overlays, cost/target/stop-loss lines
- **Market analysis** with sector money flow hierarchy (BK0 一级 → BK1 二级 → 龙头个股)
- **Interactive bubble chart** — drag to explore, zoom/pan in fullscreen, pyramid-style layout
- **AI analysis** — multi-provider support (DeepSeek, Volcano Engine, Ollama, OpenCode Zen)
- **Self-select stocks** with real-time quotes from multiple data sources (Sina, Tencent, TDX TCP)
- **Token usage tracking** per AI model
- **Dual-mode deployment**: Wails native desktop app OR self-hosted HTTP server with Basic Auth

## Screenshots

*(Insert screenshots here)*

## Quick Start

### Desktop Mode

```bash
stock-advisor.exe
```

### Web Server Mode

```bash
stock-advisor.exe -web -port=2018 -user=admin -pass=mypassword
```

Then open `http://localhost:2018` in your browser.

## Build from Source

### Prerequisites

- Go 1.21+
- Node.js 18+
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Build

```bash
git clone <repo-url> stock-advisor
cd stock-advisor
wails build -ldflags="-s -w"
```

Output: `build/bin/stock-advisor.exe`

### Development

```bash
wails dev
```

## Configuration

AI provider settings and data source priority are configured via the Settings page in-app. Settings are persisted in `data/data.json`.

Supported AI providers:
- OpenAI-compatible (DeepSeek, Ollama, OpenCode Zen, etc.)
- Volcano Engine (Doubao models)

## Data Sources

| Source | Type | Description |
|--------|------|-------------|
| `sina` | HTTP | Real-time quotes (primary; may be rate-limited) |
| `tencent` | HTTP | Real-time quotes + K-line |
| `eastmoney` | HTTP | Sector money flow, day K-line, market summary |
| `tdx` | TCP | 通达信 TCP binary protocol — most reliable |

Data source priority order is user-configurable in Settings.

## Architecture

```
stock-advisor/
├── app.go              # Wails App struct, bridge methods
├── main.go             # Entry point (desktop & web flags)
├── server.go           # HTTP server mode (SSE, REST, Basic Auth)
├── wails.json          # Wails project config
├── internal/
│   └── stock/
│       ├── eastmoney.go    # East Money API client
│       ├── sina.go         # Sina finance API client
│       ├── tencent.go      # Tencent finance API client
│       ├── tdx.go          # 通达信 TCP fetcher
│       ├── market.go       # Market summary, sector flow
│       ├── fetcher.go      # FetcherManager with priority & stats
│       ├── sector_tree.go  # Sector hierarchy (BK0→BK1→leader)
│       └── datacenter.go   # East Money datacenter API
├── frontend/
│   └── src/
│       ├── App.tsx
│       ├── api/bridge.ts   # Dual-mode Wails/Web API bridge
│       ├── pages/
│       │   ├── Home.tsx        # K-line, self-select, real-time
│       │   ├── MarketAnalysis.tsx  # Sector analysis + AI
│       │   ├── Stocks.tsx      # Self-select management
│       │   ├── AIChat.tsx      # AI chat interface
│       │   └── Settings.tsx    # AI providers, data sources
│       └── components/
│           ├── BubbleChart.tsx  # Force-directed bubble chart
│           ├── ModelIndicator.tsx
│           └── Login.tsx
└── data/
    └── data.json          # Persisted settings (auto-created)
```

## Disclaimer

**本软件仅供学习和研究使用，不构成任何投资建议。** 股市有风险，投资需谨慎。

本软件使用的行情数据来源于公开的网络接口（东方财富、新浪、腾讯等）及基于公开协议的 TCP 数据源。数据版权归原始数据提供商所有。使用者应遵守相关服务条款，不得将本软件用于商业目的。

## License

GNU General Public License v3.0
