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

// respondUserError 把 Repository 的错误译成 HTTP 状态。只有意料之外的失败才写日志。
func respondUserError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		c.JSON(404, gin.H{"error": "用户不存在"})
	case errors.Is(err, ErrUsernameTaken):
		c.JSON(409, gin.H{"error": "用户名已存在"})
	default:
		log.Println(err)
		c.JSON(500, gin.H{"error": fallback})
	}
}

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
		// 绑定到 CreateUserRequest，不是 User：请求体没有 ID，并且带校验标签。
		var req CreateUserRequest

		err := c.ShouldBindJSON(&req)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		err = CreateUser(c.Request.Context(), db, req)
		if err != nil {
			respondUserError(c, err, "创建用户失败")
			return
		}

		c.JSON(201, gin.H{
			"message": "用户创建成功",
		})
	})

	/// 注册用户查询路由
	// GET /users —— 查询所有用户
	router.GET("/users", func(c *gin.Context) {
		users, err := ListUsers(c.Request.Context(), db)
		if err != nil {
			respondUserError(c, err, "查询用户失败")
			return
		}

		var responses []UserResponse
		for _, user := range users {
			responses = append(responses, toUserResponse(user))
		}

		c.JSON(200, gin.H{
			"users": responses,
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

		user, err := GetUser(c.Request.Context(), db, id)
		if err != nil {
			respondUserError(c, err, "查询用户失败")
			return
		}

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

		err = UpdateUser(ctx, db, id, req)
		if err != nil {
			respondUserError(c, err, "更新用户失败")
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

		err = DeleteUser(ctx, db, id)
		if err != nil {
			respondUserError(c, err, "删除用户失败")
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
