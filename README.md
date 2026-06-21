# Stock Advisor

> 沪深A股智能分析桌面工具 · A-Share Stock Analysis Desktop App

---

## 简介 / Introduction

A-share (沪深A股) stock market analysis desktop application built with Wails v2 + React 18 + TypeScript + Ant Design 5 + @ant-design/charts.

开源 · 免费 · 本地运行

---

## 功能 / Features

| 中文 | English |
|------|---------|
| 📊 K线 + 资金流向图，成本/目标/止盈止损线 | K-line charts with money flow, cost/target/stop-loss lines |
| 🔍 板块资金流向（一级行业→二级行业→龙头个股） | Sector money flow hierarchy (BK0→BK1→leader stocks) |
| 🫧 交互式气泡图（力导向+金字塔全屏） | Interactive bubble chart (force-directed + fullscreen pyramid) |
| 🤖 多供应商AI分析（DeepSeek、火山、Ollama等） | Multi-provider AI (DeepSeek, Volcano, Ollama, etc.) |
| ⭐ 自选股管理 + 实时行情 | Self-select stocks with real-time quotes |
| 🔢 AI模型Token用量追踪 | Token usage tracking per AI model |
| 🖥️ 双模式：Wails桌面应用 / HTTP网页自托管 | Dual-mode: Wails desktop or self-hosted HTTP server |

---

## 快速开始 / Quick Start

### 桌面模式 / Desktop Mode

```bash
stock-advisor.exe
```

### 网页模式 / Web Server Mode

```bash
stock-advisor.exe -web -port=2018 -user=admin -pass=mypassword
```

然后浏览器打开 `http://localhost:2018`

---

## 从源码构建 / Build from Source

### 前置要求 / Prerequisites

- Go 1.21+
- Node.js 18+
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### 构建 / Build

```bash
git clone <repo-url> stock-advisor
cd stock-advisor
wails build -ldflags="-s -w"
```

输出 / Output: `build/bin/stock-advisor.exe`

### 开发 / Development

```bash
wails dev
```

---

## 配置 / Configuration

AI provider settings and data source priority are configured in-app via the Settings page (persisted in `data/data.json`).

支持的AI供应商 / Supported AI providers:
- OpenAI 兼容接口（DeepSeek、Ollama、OpenCode Zen 等）
- 火山引擎（Doubao 系列模型）

---

## 数据源 / Data Sources

| 来源 / Source | 类型 / Type | 说明 / Description |
|--------|------|-------------|
| `sina` | HTTP | 实时行情（主力，可能有频率限制） |
| `tencent` | HTTP | 实时行情 + K线 |
| `eastmoney` | HTTP | 板块资金流、日K、大盘概况 |
| `tdx` | TCP | 通达信二进制协议 — 最可靠 |

数据源优先级可在设置中调整。

---

## 项目结构 / Architecture

```
stock-advisor/
├── app.go              # Wails App 结构体、桥接方法
├── main.go             # 入口（桌面 & 网页模式）
├── server.go           # HTTP 服务器（SSE、REST、Basic Auth）
├── wails.json          # Wails 配置
├── internal/
│   └── stock/
│       ├── eastmoney.go    # 东方财富 API 客户端
│       ├── sina.go         # 新浪财经 API 客户端
│       ├── tencent.go      # 腾讯财经 API 客户端
│       ├── tdx.go          # 通达信 TCP 数据获取
│       ├── market.go       # 大盘概况、板块资金流
│       ├── fetcher.go      # 多源调度器（优先级 + 统计）
│       ├── sector_tree.go  # 板块层级树（BK0→BK1→龙头）
│       └── datacenter.go   # 东方财富数据中心 API
├── frontend/
│   └── src/
│       ├── App.tsx
│       ├── api/bridge.ts   # 双模式 API 桥接（Wails / Web）
│       ├── pages/
│       │   ├── Home.tsx        # K线、自选、实时行情
│       │   ├── MarketAnalysis.tsx  # 板块分析 + AI
│       │   ├── Stocks.tsx      # 自选股管理
│       │   ├── AIChat.tsx      # AI 对话
│       │   └── Settings.tsx    # 设置（AI供应商、数据源）
│       └── components/
│           ├── BubbleChart.tsx  # 气泡图
│           ├── ModelIndicator.tsx
│           └── Login.tsx
└── data/
    └── data.json          # 持久化配置（自动创建）
```

---

## 免责声明 / Disclaimer

**本软件仅供学习和研究使用，不构成任何投资建议。** 股市有风险，投资需谨慎。

本软件使用的行情数据来源于公开的网络接口（东方财富、新浪、腾讯等）及基于公开协议的 TCP 数据源。数据版权归原始数据提供商所有。使用者应遵守相关服务条款，不得将本软件用于商业目的。

*This software is for learning and research purposes only and does not constitute investment advice. Market data comes from public APIs. Data copyright belongs to the original providers. Do not use for commercial purposes.*

---

## 许可证 / License

GNU General Public License v3.0

---

## 交流 / Community

[GitHub Issues](https://github.com/DreamLightRain/stock-advisor/issues) · [Pull Requests](https://github.com/DreamLightRain/stock-advisor/pulls) · ✨ Star 支持
