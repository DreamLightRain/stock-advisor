import { useState } from 'react'
import { Button, Modal, Input, List, Space, Popconfirm, Typography, message, Tooltip } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, FolderOutlined } from '@ant-design/icons'
import { GetGroups, AddGroup, RemoveGroup, RenameGroup } from '../api/bridge'

const { Text } = Typography

interface Props {
  onRefresh: () => void
}

export default function GroupManager({ onRefresh }: Props) {
  const [open, setOpen] = useState(false)
  const [groups, setGroups] = useState<any[]>([])
  const [newName, setNewName] = useState('')
  const [editingName, setEditingName] = useState('')
  const [editingOld, setEditingOld] = useState('')

  const loadGroups = async () => {
    const g = await GetGroups()
    setGroups(g || [])
  }

  const handleOpen = () => {
    loadGroups()
    setOpen(true)
  }

  const handleAdd = async () => {
    if (!newName.trim()) return
    const res = await AddGroup(newName.trim())
    if (res === 'ok') {
      message.success('分组已创建')
      setNewName('')
      loadGroups()
      onRefresh()
    } else {
      message.error(res)
    }
  }

  const handleRename = async (oldName: string) => {
    if (!editingName.trim() || editingName === oldName) {
      setEditingOld('')
      setEditingName('')
      return
    }
    const res = await RenameGroup(oldName, editingName.trim())
    if (res === 'ok') {
      message.success('已重命名')
      setEditingOld('')
      setEditingName('')
      loadGroups()
      onRefresh()
    } else {
      message.error(res)
    }
  }

  const handleRemove = async (name: string) => {
    const res = await RemoveGroup(name)
    if (res === 'ok') {
      message.success('分组已删除')
      loadGroups()
      onRefresh()
    } else {
      message.error(res)
    }
  }

  return (
    <>
      <Tooltip title="管理分组">
        <Button icon={<FolderOutlined />} onClick={handleOpen}>
          分组管理
        </Button>
      </Tooltip>

      <Modal
        title="分组管理"
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={400}
      >
        <Space style={{ marginBottom: 16, width: '100%' }}>
          <Input
            placeholder="新分组名称"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            onPressEnter={handleAdd}
          />
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            添加
          </Button>
        </Space>

        <List
          dataSource={groups}
          renderItem={(item: any) => (
            <List.Item
              actions={[
                editingOld === item.name ? (
                  <Button
                    type="link"
                    size="small"
                    onClick={() => handleRename(item.name)}
                  >
                    保存
                  </Button>
                ) : (
                  <Button
                    type="link"
                    size="small"
                    icon={<EditOutlined />}
                    onClick={() => {
                      setEditingOld(item.name)
                      setEditingName(item.name)
                    }}
                  />
                ),
                <Popconfirm
                  title={`删除分组"${item.name}"？`}
                  description="分组中的股票将移到默认分组"
                  onConfirm={() => handleRemove(item.name)}
                >
                  <Button type="link" size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>,
              ]}
            >
              {editingOld === item.name ? (
                <Input
                  size="small"
                  value={editingName}
                  onChange={(e) => setEditingName(e.target.value)}
                  onPressEnter={() => handleRename(item.name)}
                  style={{ width: 200 }}
                  autoFocus
                />
              ) : (
                <Text>{item.name}</Text>
              )}
            </List.Item>
          )}
        />
      </Modal>
    </>
  )
}
