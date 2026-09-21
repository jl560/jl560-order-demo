// 对应 Go 的 UserResponse JSON，不是含 PasswordHash 的内部 User。
export type User = {
  id: number
  username: string
  age: number
}

export type UsersResponse = {
  users: User[]
}

export type UserWritePayload = {
  username: string
  password: string
  age: number
}

export type ApiError = {
  error: string
}
