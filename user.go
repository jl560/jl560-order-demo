package main

/*
三种类型为什么要分开：

	它们的变化原因不同。
	- 请求体：由前端契约决定（能提交什么、怎么校验）
	- User：由数据库表结构决定（存什么）
	- 响应体：由「客户端允许看见什么」决定

	Phase 1 里一个 User 身兼三职，造成三个真实问题：
	GET 返回明文密码、库里存明文、客户端能在 JSON 里塞 id。
*/

// CreateUserRequest 是 POST /users 的请求体。没有 ID：主键由数据库自增。
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6"`
	Age      int    `json:"age" binding:"gte=0,lte=150"`
}

// UpdateUserRequest 是 PUT /users/:id 的请求体。id 走路径参数，不走 JSON。
type UpdateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=32"`
	Password string `json:"password" binding:"required,min=6"`
	Age      int    `json:"age" binding:"gte=0,lte=150"`
}

// User 对应 users 表的一行。PasswordHash 是 bcrypt 哈希，不是用户输入的明文。
type User struct {
	ID           int
	Username     string
	PasswordHash string
	Age          int
}

// UserResponse 是返回给客户端的用户。故意没有密码字段。
type UserResponse struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Age      int    `json:"age"`
}

func toUserResponse(user User) UserResponse {
	return UserResponse{
		ID:       user.ID,
		Username: user.Username,
		Age:      user.Age,
	}
}
