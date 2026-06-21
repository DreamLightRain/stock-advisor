import { useState } from 'react'
import { Card, Form, Input, Button, Typography, message, Alert } from 'antd'
import { SafetyOutlined } from '@ant-design/icons'
import { isWebMode, saveCredentials } from '../api/auth'

const { Title, Text } = Typography

export default function Login({ onLogin }: { onLogin: () => void }) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (values: { user: string; pass: string }) => {
    if (!isWebMode()) {
      onLogin()
      return
    }

    setLoading(true)
    setError('')

    try {
      // Test credentials by calling a simple API
      const resp = await fetch('/api/call', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Basic ' + btoa(values.user + ':' + values.pass),
        },
        body: JSON.stringify({ method: 'GetSettings', args: [] }),
      })

      if (resp.status === 401) {
        setError('用户名或密码错误')
        setLoading(false)
        return
      }

      if (!resp.ok) {
        setError('服务器错误: ' + resp.status)
        setLoading(false)
        return
      }

      saveCredentials(values.user, values.pass)
      message.success('登录成功')
      onLogin()
    } catch (e: any) {
      setError('无法连接服务器: ' + (e.message || String(e)))
    }
    setLoading(false)
  }

  return (
    <div style={{
      height: '100vh',
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      background: '#f0f2f5',
    }}>
      <Card style={{ width: 400, boxShadow: '0 2px 8px rgba(0,0,0,0.1)' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <SafetyOutlined style={{ fontSize: 48, color: '#1890ff' }} />
          <Title level={3} style={{ marginTop: 16, marginBottom: 4 }}>股票分析系统</Title>
          <Text type="secondary">请输入访问凭证</Text>
        </div>

        {error && (
          <Alert type="error" message={error} showIcon style={{ marginBottom: 16 }} closable onClose={() => setError('')} />
        )}

        <Form layout="vertical" onFinish={handleSubmit} autoComplete="off">
          <Form.Item label="用户名" name="user" rules={[{ required: true, message: '请输入用户名' }]}>
            <Input placeholder="admin" size="large" />
          </Form.Item>
          <Form.Item label="密码" name="pass" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password placeholder="密码" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block size="large">
              登录
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
