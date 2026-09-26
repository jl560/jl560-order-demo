//⚠⚠⚠router.go 里既有 Router，也有 Handler。
// 真正“执行一次 HTTP 请求”的，是 Handler。
// Handler 再去调用 user_service.go 里的业务函数。

package main

import (
	//提供访问关系型数据库的通用接口
	//（SQL = Structured Query Language，结构化查询语言）
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	//提供字符串与基本数据类型之间的转换，
	//如 Atoi、Itoa、ParseInt 等（strconv = string conversion，字符串转换）
	"strconv"

	// 第三方包
	// Gin：第三方 Go Web 框架（Gin 是框架名称，不是缩写，取 Gin 酒之意，意为"辛辣、快速"）
	/*
		★ 【Gin 路由与请求处理通用机制说明】（适用于本文件所有路由）

		1. 路由注册（“贴便利贴”）：
			router.GET()、router.POST() 等方法在程序启动时被调用，
			它们并不会执行后面的处理函数，而是把“路由规则”和“处理函数”
			登记到 Gin 内部的路由树中。整个过程相当于在内存里建立了一张查报表。

		2. 路由模板 vs 真实 URL：
			"/users/:id" 是一个路由匹配模板（规则），其中的 :id 是动态占位符。
			它可以匹配无数个真实 URL，如 /users/1、/users/999。
			当用户访问这些真实 URL 时，Gin 才会根据模板匹配到对应的处理函数。

		3. 执行时机：
			程序启动 -> 执行完所有路由注册 -> 跑到 router.Run() 阻塞监听
			-> 用户访问真实 URL -> Gin 匹配路由树 -> 执行对应处理函数。

		4. c *gin.Context 上下文对象：
			这是 Gin 为每个请求创建的“百宝箱”，封装了：
				- 请求信息（请求头、请求体、路径参数、查询参数）
				- 响应输出方法（c.JSON()、c.String() 等）
			通过它可以方便地读取客户端数据并返回响应。

		5. gin.H 底层原理：
			gin.H 是 map[string]interface{} 的类型别名，
			用于快速构造 JSON 对象，c.JSON() 会自动将其序列化成 JSON 字符串。

		6. 参数化查询防 SQL 注入：
			使用 $1、$2 等占位符，并将实际参数作为单独参数传入，
			数据库驱动会正确处理转义，从根本上杜绝 SQL 注入风险。

		7. Context 取消机制：
			c.Request.Context() 返回与请求绑定的 context.Context，
			当客户端断开连接或超时时，该 ctx 会被取消，
			后续数据库操作（如 QueryRowContext）会立刻中止，节省服务器资源。

		★ 后续每个具体路由的注释将专注于自身业务逻辑，通用原理不再重复。
	*/
	"github.com/gin-gonic/gin"
)

func SetupRouter(db *sql.DB) *gin.Engine {

	///// 创建 Gin 路由器
	router := gin.Default()

	// 页面在 localhost:5173，直连 :8080 时由这里统一允许跨源读取。
	// OPTIONS 是浏览器在 POST/PUT/DELETE 之前的预检，这里直接返回，不进入业务 Handler。
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 前端改由 Vite 在 :5173 开发。这里只提供 API，不再托管 HTML。

	/*
		router.GET("/ping", ...)

		GET：
		HTTP 请求方法之一，常用于获取资源。

		"/ping"：
		请求路径。

		func(c *gin.Context) { ... }：
		当客户端发送 GET /ping 请求时，
		Gin 就执行这个处理函数（Handler）。

		c 是 Gin 为当前 HTTP 请求提供的 Context 对象，
		可以通过它获取请求信息、返回响应等。
	*/
	router.GET("/ping", func(c *gin.Context) {

		// 返回 HTTP 200 状态码，并返回一个 JSON 响应
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	/// 注册用户创建路由
	// POST /users —— 处理客户端提交的新用户注册请求
	router.POST("/users", func(c *gin.Context) {
		/*
			Gin 上下文（c）：由 Gin 框架传入，封装了本次 HTTP 请求的所有信息
			包括请求头、请求体、路径参数、响应写入器等。用于绑定 JSON、返回响应等操作
		*/

		// 绑定到 CreateUserRequest，不是 User：请求体没有 ID，并且带校验标签。
		var req CreateUserRequest

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		/*
			第 1 步：从 Gin 请求中提取标准库 Context

			ctx := c.Request.Context() 将当前 HTTP 请求的上下文提取出来，
			赋值给变量 ctx。

			这个 ctx 与本次 HTTP 请求“同生共死”：
				- 正常情况：请求处理完，ctx 自动结束
				- 异常情况：用户关闭浏览器、断网、或超时，ctx 会被立刻取消

			第 2 步：将 ctx 传给数据库操作（db.ExecContext）

			把 ctx 作为第一个参数传给 ExecContext，相当于给数据库操作
			接了一根“电话线”：
				- 如果用户一直正常，SQL 正常执行，返回结果
				- 如果用户中途断开，ctx 发出取消信号，ExecContext 底层
					会通过 TCP 连接向 PostgreSQL 发送取消请求，中断正在执行的 SQL

			这样做的好处：避免数据库还在傻等，浪费 CPU 和连接资源。

			第 3 步：参数化查询（$1、$2、$3 占位符）

			`INSERT INTO users ... VALUES ($1, $2, $3)` 是一段 SQL 语句，
			它本身属于 PostgreSQL 的 SQL 方言语法（$n 是 PostgreSQL 特有的占位符写法）。
			在 Go 代码里，它只是作为一个字符串参数传给 db.ExecContext() 函数，
			Go 编译器不会解析这段 SQL，而是把它原样发送给 PostgreSQL 服务器去解析和执行。

			$1、$2、$3 是参数占位符，分别对应后面传入的：
				req.Username     → $1
				password_hash    → $2（bcrypt 之后，不是明文）
				req.Age          → $3

			为什么不用字符串拼接？
				因为拼接会被 SQL 注入攻击，比如用户输入 ' OR '1'='1。
				用占位符 + 参数传递，Go 驱动会把参数当“纯数据值”传给数据库，
				不会当成 SQL 代码执行，彻底杜绝注入风险。

			第 4 步：返回值处理

			db.ExecContext 返回两个值：
				第一个（用 _ 忽略）：sql.Result 类型，包含插入的行数、自增 ID 等信息
				第二个（赋值给 err）：error 类型，如果有错误则非 nil

			当前注册接口不需要返回插入后的 ID，所以用 _ 丢弃第一个返回值，
			只关心 err 是否为 nil。

			第 5 步：错误处理与响应

			如果 err != nil（比如用户名重复、数据库连接断开等），
			则调用 c.JSON() 向客户端返回 HTTP 响应。

			c.JSON() 是 Gin 框架提供的方法，作用是把数据序列化成 JSON 格式，
			放进 HTTP 响应体里，发回给客户端。

			gin.H 是 Gin 框架定义的一个类型别名：
				本质是 map[string]interface{}（即键为字符串、值为任意类型的字典）。
				它只是为了让写 JSON 数据时更简短，比如：
					gin.H{"error": "创建用户失败"}
				等价于：
					map[string]interface{}{"error": "创建用户失败"}

			c.JSON(500, gin.H{"error": "创建用户失败"}) 的含义：
				1. 状态码 500：告诉客户端“服务器内部错误”
				2. 数据体 {"error":"创建用户失败"}：告诉客户端具体的错误原因
				客户端收到后可以在前端展示这个错误信息。
		*/
		ctx := c.Request.Context()

		err = CreateUser(ctx, db, req)
		if err != nil {
			// 日志落地。这行会把 Service 包装好的 "创建用户失败: xxx" 打印到终端。
			// 在这里用 log.Println 给开发者看。
			log.Println("CreateUser error: - router.go:185", err)

			c.JSON(500, gin.H{
				"error": "创建用户失败",
			})
			return
		}

		c.JSON(201, gin.H{
			"message": "用户创建成功",
		})
	})

	/// 注册用户查询路由
	// GET /users —— 查询所有用户
	router.GET("/users", func(c *gin.Context) {
		/*
			获取当前 HTTP 请求对应的标准库 Context。
			这个 ctx 用于控制超时和取消，若客户端断开连接，数据库查询将被中断。
		*/
		ctx := c.Request.Context()

		/*
			查询 users 表，按 id 升序返回所有记录。
			db.QueryContext() 会返回一个 *sql.Rows 结果集，以及可能的错误。
		*/
		rows, err := db.QueryContext(
			ctx,
			`SELECT id, username, password_hash, age
		 FROM users
		 ORDER BY id`,
		)
		if err != nil {
			/*
				如果查询失败（例如数据库连接断开、SQL 语法错误等），返回 500。
				500 Internal Server Error 表示服务器内部处理出错，客户端无法解决。
			*/
			c.JSON(500, gin.H{
				"error": "查询用户失败",
			})
			return
		}

		/*
			defer rows.Close() 确保在函数返回前释放数据库连接。
			如果不关闭，连接会一直被占用，导致连接池耗尽。
			rows 对象内部持有数据库连接，必须在遍历完后显式关闭。
		*/
		defer rows.Close()

		// 用来保存最终查询到的所有用户
		var users []UserResponse

		/*
			【游标（指针）机制详解】

			rows 本身是一个指针（游标），指向结果集的当前位置，但初始时不指向任何有效行。

			rows.Next() 的作用是让这个指针向下移动一行：
			- 如果移动后指向有效数据，则返回 true
			- 如果已经移到最后一行之后（无更多数据），则返回 false

			它相当于“翻书页”，只改变书签（指针）的位置，并不读取内容。
			每次调用前必须确保 rows 没有被关闭。
		*/
		/*
			★ rows *sql.Rows 内存模型

			rows（变量）
			  │
			  │ 是一个指针，保存了一个内存地址
			  ▼
			┌───────────────────────────────────────────────────┐
			│                   sql.Rows 对象                   │
			│  ┌─────────────────────────────────────────────┐  │
			│  │  游标位置（当前行索引）                       │  │
			│  │  例如：当前指向第 2 行                        │  │
			│  │  rows.Next() 让游标 +1                       │  │
			│  │  rows.Scan() 读取游标所指行的数据             │  │
			│  ├─────────────────────────────────────────────┤  │
			│  │  列信息：                                    │  │
			│  │  - 列名：[id, username, password_hash, age]   │  │
			│  │  - 列类型：[int, string, string, int]        │  │
			│  ├─────────────────────────────────────────────┤  │
			│  │  连接引用：指向 db 连接池中的一个连接          │  │
			│  ├─────────────────────────────────────────────┤  │
			│  │  数据缓冲区：预读的部分行数据                  │  │
			│  │  （减少网络往返，提升性能）                    │  │
			│  ├──────────────────────────────────────────────┤  │
			│  │  错误状态：记录遍历过程中发生的错误             │  │
			│  │  （通过 rows.Err() 获取）                     │  │
			│  └──────────────────────────────────────────────┘  │
			└────────────────────────────────────────────────────┘

			【与 C++ 的对比】
			Go:  rows.Next() + rows.Scan()
			      ↓
			C++:  iterator++ + *iterator

			【关键理解】
			rows.Next()  → 只移动游标（翻书签），不读取数据
			rows.Scan()  → 只读取数据（读书签所在页），不移动游标
			两者各司其职，配合完成逐行遍历。
		*/
		for rows.Next() {
			var user User

			/*
				rows.Scan() 的作用：将当前行的各列数据依次赋值给传入的变量（指针）。

				关键理解：
				- Scan() 本身不会移动游标，它只读取“当前游标所指的那一行”的数据。
				- 所以必须先用 rows.Next() 把游标移到有效行，再调用 Scan()。
				- 传入 &user.ID 是因为 Scan 需要知道变量的内存地址，才能把读取的值写入该地址。
				- 如果只传 user.ID（值），Scan 只能拿到副本，无法修改原变量。

				参数顺序必须与 SELECT 子句中的列顺序完全一致。
				如果列类型与变量类型不匹配，或某列为 NULL 且变量不可接受 NULL，则会返回错误。
			*/
			/*
				rows.Scan 的内部实现，本质上就是通过反射解引用指针（*ptr = 从数据库读取的值），
				它将当前行的数据直接写入传入的地址所指向的内存位置，
				与用 C/C++ 通过指针修改函数外部变量的逻辑完全一致
				——只不过在 Go 里，我们需要显式传 & 取地址，
				Scan 内部替我们完成了“拿到地址 → 写入值”这一整套操作。
			*/
			err := rows.Scan(
				&user.ID,
				&user.Username,
				&user.PasswordHash,
				&user.Age,
			)
			if err != nil {
				/*
					Scan 错误可能因为类型转换失败、列数不匹配等，返回 500。
					500 Internal Server Error 表示服务器内部处理出错。
				*/
				c.JSON(500, gin.H{
					"error": "读取用户数据失败",
				})
				return
			}

			// 将当前用户追加到切片中
			users = append(users, toUserResponse(user))
		}

		/*
			遍历结束后，必须检查 rows.Err() 以发现遍历过程中的错误。

			原因：
			rows.Next() 返回 false 可能有两种情况：
			1. 正常遍历到了最后一行（无错误）
			2. 遍历过程中发生了网络中断、驱动内部错误等

			rows.Err() 就是用来区分这两种情况的。
			如果是情况 2，我们同样返回 500，告诉前端服务器内部出问题了。
		*/
		if err := rows.Err(); err != nil {
			c.JSON(500, gin.H{
				"error": "读取用户数据失败",
			})
			return
		}

		/*
			返回用户列表，状态码 200 OK。
			gin.H{"users": users} 将切片序列化为 JSON 数组。
		*/
		c.JSON(200, gin.H{
			"users": users,
		})
	})

	/*
	   注册单个用户查询路由（GET /users/:id）

	   【路由匹配】
	   /users/:id 是动态模板，:id 是路径参数。
	   它可以匹配 /users/1、/users/999 等无数个真实 URL。
	   通用机制见上方《Gin 路由与请求处理通用机制说明》。
	*/
	router.GET("/users/:id", func(c *gin.Context) {
		/*
			1. 从路径中取出 id 字符串。
			例如：用户访问 /users/5，idStr 得到 "5"。
			c.Param("id") 中的 "id" 必须与路由模板中的占位符名称 :id 保持一致。
		*/
		idStr := c.Param("id")

		/*
			2. 将字符串转为整数，若格式错误返回 400。

			400 Bad Request 表示客户端请求参数有误：
			- 如果用户传了非数字（如 /users/abc），Atoi 会报错
			- 此时返回 400 告诉客户端参数格式不对，客户端需要修正后再试
		*/
		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "用户 ID 格式错误",
			})
			return
		}

		/*
			3. 获取请求上下文（用于超时/取消控制）。
			若客户端断开连接，ctx 会被取消，数据库查询会终止，节省服务器资源。
		*/
		ctx := c.Request.Context()

		/*
			4. 执行参数化查询（单条记录）。

			【与 QueryContext 的对比】
			QueryRowContext 返回的是 *sql.Row 而不是 *sql.Rows。
			它内部也维护了一个隐形的游标，但只指向查询结果的第一行（且仅此一行）。
			调用 .Scan() 时，这个隐式游标已经被定位到了唯一的那一行数据上，
			因此不需要再调用 Next()，直接读取即可。

			【特殊情况】
			若没有找到匹配的记录，Scan 会返回 sql.ErrNoRows。
			若有多条匹配（实际上 id 是主键，不会发生），也只取第一行。

			【防 SQL 注入】
			使用 $1 占位符 + 参数传递的方式，而不是直接拼接 SQL 字符串。
			数据库驱动会把参数当作“纯数据值”处理，不会当成 SQL 代码执行，
			从根本上杜绝了 SQL 注入攻击。
		*/
		var user User
		err = db.QueryRowContext(
			ctx,
			`SELECT id, username, password_hash, age
			FROM users
			WHERE id = $1`,
			id,
		).Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Age,
		)

		/*
			5. 处理查询错误。

			情况一：sql.ErrNoRows
			- 表示没有匹配的记录，即该 ID 在数据库中不存在
			- 属于“资源不存在”，返回 404 Not Found

			情况二：其他错误
			- 如数据库连接失败、网络超时、驱动内部错误等
			- 这些都属于服务器内部处理出错，客户端无法自行解决
			- 返回 500 Internal Server Error
		*/
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				c.JSON(404, gin.H{"error": "用户不存在"})
				return
			}
			c.JSON(500, gin.H{"error": "查询用户失败"})
			return
		}

		/*
			6. 查询成功，返回用户信息。

			- 状态码 200 OK 表示请求成功
			- Gin 自动将 UserResponse 序列化为 JSON
			- 没有 password 字段，哈希不会离开服务器
		*/
		c.JSON(200, toUserResponse(user))
	})

	/// 注册用户修改路由
	// PUT /users/:id —— 根据用户 ID 修改用户
	router.PUT("/users/:id", func(c *gin.Context) {
		idStr := c.Param("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "用户 ID 格式错误",
			})
			return
		}

		var req UpdateUserRequest

		err = c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		ctx := c.Request.Context()

		updated, err := UpdateUser(ctx, db, id, req)
		if err != nil {
			log.Println("UpdateUser error: - router.go:486", err)
			c.JSON(500, gin.H{
				"error": "更新用户失败",
			})
			return
		}

		if !updated {
			c.JSON(404, gin.H{
				"error": "用户不存在",
			})
			return
		}

		c.JSON(200, gin.H{
			"message": "用户更新成功",
		})
	})

	///注册用户删除路由
	// DELETE /users/:id —— 根据用户 ID 删除用户
	router.DELETE("/users/:id", func(c *gin.Context) {
		idStr := c.Param("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "用户 ID 格式错误",
			})
			return
		}

		ctx := c.Request.Context()

		deleted, err := DeleteUser(ctx, db, id)
		if err != nil {
			log.Println("DeleteUser error: - router.go:522", err)
			c.JSON(500, gin.H{
				"error": "删除用户失败",
			})
			return
		}

		if !deleted {
			c.JSON(404, gin.H{
				"error": "用户不存在",
			})
			return
		}

		c.JSON(200, gin.H{
			"message": "用户删除成功",
		})
	})

	/*
		/test-timeout —— 超时取消（救服务器不卡死）

		机制：context.WithTimeout 创建 2 秒寿命的 ctx，同时 time.After 模拟 5 秒慢任务。
		select 同时等这两个信号，2 秒先到，立即返回 408，不等 5 秒。

		工程意义：防止慢 SQL 或慢接口堵死连接池。
		这叫“快速失败”——宁可用 2 秒报错，也不卡 10 秒拖垮整个服务。

		并发理解：在等这 5 秒的过程中，CPU 没有白等，它把 CPU 时间让给了其他请求。
		超时信号一到，立刻唤醒这个 goroutine 执行取消分支。
	*/
	router.GET("/test-timeout", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		select {
		case <-time.After(5 * time.Second):
			c.JSON(200, gin.H{
				"message": "任务完成",
			})

		case <-ctx.Done():
			c.JSON(408, gin.H{
				"error": ctx.Err().Error(),
			})
		}
	})

	// 测试 goroutine —— 展示后台任务不阻塞 HTTP 响应
	/*
		/test-goroutine —— 后台任务与请求绑定（省用户的等待时间，防泄漏）

		机制：go func 启动后台任务，每隔 0.5 秒打印“正在工作”。
		HTTP 主线程不等待，立即返回 200。
		用户关闭浏览器时，ctx 被取消，goroutine 通过 <-ctx.Done() 感知到，自己 return。

		工程意义：用户关掉页面，后台任务立刻停止，不浪费 CPU 和内存。
		这叫“协作式取消”——Context 不杀 goroutine，只发信号，goroutine 自己决定安全退出。

		并发理解：“先给我发个消息说收到了，然后它再去干耗时的事”。
		HTTP 已经返回了，用户不白等；后台还在跑，但人可以随时掐断它。
	*/
	router.GET("/test-goroutine", func(c *gin.Context) {
		// 获取请求上下文
		ctx := c.Request.Context()

		// 启动一个 goroutine（后台任务）
		go func() {
			for {
				select {
				case <-ctx.Done():
					// 收到取消信号（请求结束/超时），打印退出信息
					fmt.Println("goroutine：收到取消信号，退出 - router.go:594")
					return

				default:
					// 没有取消信号，继续工作
					fmt.Println("goroutine：正在工作 - router.go:599")
					time.Sleep(500 * time.Millisecond) // 等待0.5秒
				}
			}
		}()

		// HTTP 立即返回，不等待 goroutine 结束
		c.JSON(200, gin.H{
			"message": "HTTP请求已经返回",
		})
	})

	/*
		/test-waitgroup —— 等待并发任务全部完成（收拢任务算总账）

		机制：wg.Add(3) 设计数器为 3；三个 goroutine 分别睡 1、2、3 秒；
		每个结束前 defer wg.Done() 使计数器减 1；wg.Wait() 阻塞直到计数器归零。

		工程意义：启动时需要并发加载多个配置，或一个请求需要聚合多个第三方 API 结果，
		必须等所有并发任务收拢完毕再往下走。

		注意：Add 的数量必须严格等于启动的 goroutine 数量，多一个会泄漏，少一个会死锁。

		并发理解：三个任务同时开始等，总耗时 = 3 秒（最长那个），而不是 1+2+3=6 秒。
		这就是并发节省“人的等待时间”的直接体现。
	*/
	router.GET("/test-waitgroup", func(c *gin.Context) {
		var wg sync.WaitGroup

		wg.Add(3)

		go func() {
			defer wg.Done()
			fmt.Println("任务A开始 - router.go:632")
			time.Sleep(1 * time.Second)
			fmt.Println("任务A结束 - router.go:634")
		}()

		go func() {
			defer wg.Done()
			fmt.Println("任务B开始 - router.go:639")
			time.Sleep(2 * time.Second)
			fmt.Println("任务B结束 - router.go:641")
		}()

		go func() {
			defer wg.Done()
			fmt.Println("任务C开始 - router.go:646")
			time.Sleep(3 * time.Second)
			fmt.Println("任务C结束 - router.go:648")
		}()

		wg.Wait()

		c.JSON(200, gin.H{
			"message": "三个任务全部完成",
		})
	})
	/*
		三大实验的递进关系（总纲）

		1. test-timeout   → 学会了「怎么发出取消信号」。
		2. test-goroutine → 学会了「goroutine 怎么收到信号后自己退出」。
		3. test-waitgroup → 学会了「怎么等所有 goroutine 都退出再继续」。

		三者叠加 = Go 并发控制的完整闭环：
		启动任务（goroutine）→ 传递控制（Context）→ 同步等待（WaitGroup）。

		这就是将来写任何高并发后端服务的“骨架”。
	*/

	/*
		  /test-channel —— 并发任务 + 管道传数据 + 等待完成 + 取消控制

		【这个接口想证明什么】
		同时派 3 个工人干活（分别耗时 1、2、3 秒），
		用管道把每个工人的“成果”传回主程序，
		用 WaitGroup 确保 3 个人都干完了再收工，
		用 Context 确保老板喊停时工人立刻跑路。

		【四个角色的分工】
		- goroutine  → 干活的人
		- channel    → 传送带（传数据，不负责排序）
		- WaitGroup  → 计数本（等所有人干完）
		- Context    → 老板的喇叭（喊停）

		【并发理解】
		三个任务同时开始，总耗时 = 最长那个（3秒），而不是 1+2+3=6 秒。
		这就是“并发省人的等待时间”的直接体现。
	*/
	router.GET("/test-channel", func(c *gin.Context) {
		// 1. 准备工作：拿取消信号 + 造管道 + 设计数器
		ctx := c.Request.Context()      // 跟请求绑定的取消信号
		results := make(chan string, 3) // 造一根能传 3 个字符串的管道（缓冲为 3）
		var wg sync.WaitGroup           // 造一个计数本
		wg.Add(3)                       // 本子上记 3 笔账（有 3 个活要干）

		// 2. Worker 1（耗时 1 秒）
		go func() {
			defer wg.Done() // 临走前跟老板说一声“我干完了”（计数器减 1）

			time.Sleep(1 * time.Second) // 模拟干活（查库、调接口等）

			// select：同时监听两件事
			select {
			case results <- "任务1完成": // 如果管道没满，把结果扔进去
			case <-ctx.Done(): // 如果老板喊停（请求取消/超时）
				return // 立刻收工回家，不扔结果了
			}
		}()

		// 3. Worker 2（耗时 2 秒）—— 结构和 Worker 1 完全一样，只是睡 2 秒
		go func() {
			defer wg.Done()
			time.Sleep(2 * time.Second)
			select {
			case results <- "任务2完成":
			case <-ctx.Done():
				return //好习惯
			}
		}()

		// 4. Worker 3（耗时 3 秒）—— 结构和 Worker 1 完全一样，只是睡 3 秒
		go func() {
			defer wg.Done()
			time.Sleep(3 * time.Second)
			select {
			case results <- "任务3完成":
			case <-ctx.Done():
				return //好习惯
				// 如果不写 return，程序也会继续往下执行，若select后面有代码，会执行被CTX取消过后本不应该再执行的任务
			}
		}()

		// 5. 等待所有 Worker 完成（老板坐门口死等）
		wg.Wait() // 计数器归零之前，这行代码卡着不动

		// 6. 收摊子：关掉传送带 + 把管道里所有结果取出来
		close(results) // 关掉管道（告诉 for range：不会再有人扔东西进来了）

		var output []string           // 准备一个空袋子
		for result := range results { // 循环从管道里取东西，直到管道被关闭且取空
			output = append(output, result) // 每取一个就装进袋子
		}

		// 7. 把袋子里的结果返回给浏览器
		c.JSON(200, gin.H{
			"results": output, // 输出顺序由任务实际完成顺序决定，不由代码顺序决定
		})
	})

	return router
}
