import { useState, useEffect } from 'react'
import { Layout, Menu, Typography } from 'antd'
import {
  HomeOutlined,
  SearchOutlined,
  RobotOutlined,
  SettingOutlined,
  BarChartOutlined,
  FileTextOutlined,
} from '@ant-design/icons'
import Home from './pages/Home'
import Stocks from './pages/Stocks'
import AIChat from './pages/AIChat'
import Settings from './pages/Settings'
import MarketAnalysis from './pages/MarketAnalysis'
import Logs from './pages/Logs'
import Login from './pages/Login'
import ErrorBoundary from './components/ErrorBoundary'
import { isWebMode, getCredentials } from './api/auth'

const { Sider, Content } = Layout

const pages = [
  { key: 'home', label: '自选首页', icon: <HomeOutlined /> },
  { key: 'stocks', label: '股票搜索', icon: <SearchOutlined /> },
  { key: 'analysis', label: '市场分析', icon: <BarChartOutlined /> },
  { key: 'ai', label: 'AI分析', icon: <RobotOutlined /> },
  { key: 'logs', label: '日志记录', icon: <FileTextOutlined /> },
  { key: 'settings', label: '设置', icon: <SettingOutlined /> },
]

function App() {
  const [current, setCurrent] = useState('home')
  const [collapsed, setCollapsed] = useState(false)
  const [authenticated, setAuthenticated] = useState(!isWebMode() || !!getCredentials())

  const renderPage = () => {
    switch (current) {
      case 'home': return <Home />
      case 'stocks': return <Stocks />
      case 'analysis': return <MarketAnalysis />
      case 'ai': return <AIChat />
      case 'logs': return <Logs />
      case 'settings': return <Settings />
      default: return <Home />
    }
  }

  if (!authenticated) {
    return <Login onLogin={() => setAuthenticated(true)} />
  }

  return (
    <Layout style={{ height: '100vh', overflow: 'hidden' }}>
      <Sider
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        theme="light"
        style={{
          borderRight: '1px solid #f0f0f0',
          boxShadow: '2px 0 8px rgba(0,0,0,0.05)',
          overflow: 'hidden',
        }}
      >
        <div style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid #f0f0f0',
        }}>
          <Typography.Title level={4} style={{ margin: 0, fontSize: collapsed ? 16 : 18 }}>
            {collapsed ? '📈' : '📈 智能股票'}
          </Typography.Title>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[current]}
          items={pages}
          onClick={({ key }) => setCurrent(key)}
          style={{ borderRight: 0 }}
        />
      </Sider>
      <Layout style={{ overflow: 'auto' }}>
        <Content style={{
          margin: 16,
          padding: 20,
          background: '#fff',
          borderRadius: 8,
          minHeight: 280,
        }}>
          <ErrorBoundary key={current}>
            {renderPage()}
          </ErrorBoundary>
        </Content>
      </Layout>
    </Layout>
  )
}

export default App
