import type { ApiError, User, UserWritePayload, UsersResponse } from '../types/user.ts'

// 开发时走相对路径，由 Vite 代理到 Go。改地址只动这一处。
const API_BASE = ''

async function readApiError(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as ApiError
    if (body && body.error) {
      return body.error
    }
  } catch {
    // 响应不是 JSON
  }
  return '请求失败（HTTP ' + response.status + '）'
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  const response = await fetch(API_BASE + path, init)
  if (!response.ok) {
    throw new Error(await readApiError(response))
  }
  return response
}

export async function listUsers(): Promise<User[]> {
  const response = await request('/users')
  const data = (await response.json()) as UsersResponse
  return data.users || []
}

export async function createUser(payload: UserWritePayload): Promise<void> {
  await request('/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export async function updateUser(id: number, payload: UserWritePayload): Promise<void> {
  await request('/users/' + id, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
}

export async function deleteUser(id: number): Promise<void> {
  await request('/users/' + id, { method: 'DELETE' })
}
