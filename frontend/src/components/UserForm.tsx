type UserFormProps = {
  username: string
  password: string
  age: string
  editingId: number | null
  onUsernameChange: (value: string) => void
  onPasswordChange: (value: string) => void
  onAgeChange: (value: string) => void
  onSubmit: () => void
  onCancel: () => void
}

export function UserForm(props: UserFormProps) {
  return (
    <section>
      <h2>{props.editingId ? '编辑用户 #' + props.editingId : '创建用户'}</h2>
      <form
        onSubmit={(event) => {
          event.preventDefault()
          props.onSubmit()
        }}
      >
        <label>
          用户名
          <input
            type="text"
            autoComplete="username"
            required
            minLength={3}
            maxLength={32}
            value={props.username}
            onChange={(event) => props.onUsernameChange(event.target.value)}
          />
        </label>
        <label>
          密码
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={6}
            value={props.password}
            onChange={(event) => props.onPasswordChange(event.target.value)}
          />
        </label>
        <label>
          年龄
          <input
            type="number"
            min={0}
            max={150}
            required
            value={props.age}
            onChange={(event) => props.onAgeChange(event.target.value)}
          />
        </label>
        <button type="submit">{props.editingId ? '保存修改' : '创建'}</button>
        {props.editingId ? (
          <button type="button" onClick={props.onCancel}>
            取消编辑
          </button>
        ) : null}
      </form>
    </section>
  )
}
