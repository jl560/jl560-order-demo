import { useEffect, useState } from 'react'
import { createUser, deleteUser, listUsers, updateUser } from './api/users.ts'
import { UserForm } from './components/UserForm.tsx'
import { UserList } from './components/UserList.tsx'
import type { User, UserWritePayload } from './types/user.ts'

export default function App() {
  const [users, setUsers] = useState<User[]>([])
  const [error, setError] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [age, setAge] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)

  async function refreshList() {
    const list = await listUsers()
    setUsers(list)
  }

  useEffect(() => {
    void refreshList().catch((err: unknown) => {
      setError(err instanceof Error ? err.message : String(err))
    })
  }, [])

  function resetForm() {
    setUsername('')
    setPassword('')
    setAge('')
    setEditingId(null)
  }

  async function handleSubmit() {
    setError('')
    const payload: UserWritePayload = {
      username: username.trim(),
      password,
      age: Number(age),
    }
    try {
      if (editingId !== null) {
        await updateUser(editingId, payload)
      } else {
        await createUser(payload)
      }
      resetForm()
      await refreshList()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  async function handleDelete(id: number) {
    setError('')
    try {
      await deleteUser(id)
      if (editingId === id) {
        resetForm()
      }
      await refreshList()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <>
      <h1>用户管理</h1>
      {import.meta.env.DEV ? (
        <p className="hint">
          开发页面在 Vite :5173，请求发往 http://127.0.0.1:8080。
        </p>
      ) : null}
      <UserForm
        username={username}
        password={password}
        age={age}
        editingId={editingId}
        onUsernameChange={setUsername}
        onPasswordChange={setPassword}
        onAgeChange={setAge}
        onSubmit={() => void handleSubmit()}
        onCancel={resetForm}
      />
      {error ? <p className="error">{error}</p> : null}
      <UserList
        users={users}
        onEdit={(user) => {
          setUsername(user.username)
          setAge(String(user.age))
          setPassword('')
          setEditingId(user.id)
          setError('')
        }}
        onDelete={(id) => void handleDelete(id)}
      />
    </>
  )
}
