import type { User } from '../types/user.ts'

type UserListProps = {
  users: User[]
  onEdit: (user: User) => void
  onDelete: (id: number) => void
}

export function UserList(props: UserListProps) {
  return (
    <section>
      <h2>用户列表</h2>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>用户名</th>
            <th>年龄</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {props.users.length === 0 ? (
            <tr>
              <td colSpan={4}>暂无用户</td>
            </tr>
          ) : (
            props.users.map((user) => (
              <tr key={user.id}>
                <td>{user.id}</td>
                <td>{user.username}</td>
                <td>{user.age}</td>
                <td>
                  <button type="button" onClick={() => props.onEdit(user)}>
                    编辑
                  </button>{' '}
                  <button type="button" onClick={() => props.onDelete(user.id)}>
                    删除
                  </button>
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </section>
  )
}
