package main

import (
	// Go 标准库：安装 Go 环境后自带，不需要额外下载

	//提供格式化输入输出，如 Println、Printf、Sprintf（fmt = format，格式化）
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	//提供日志输出和 Fatal 等错误处理工具（log = logging，日志记录）
	"log"
)

func main() {

	// 第0步：加载配置
	/*
		这一步是新加的，位置必须在最前面。

		它做两件事（细节见 config.go）：
			1. 如果存在 .env 文件，把里面的键值对读进进程的环境变量表
			2. 从环境变量读出配置，填进 Config 结构体

		它是整个程序里唯一一个"在碰数据库之前就可能让程序退出"的关卡。
		这道关卡的价值在于失败得早、失败得清楚：
			改之前，密码配错的表现是走到 db.Ping() 才收到一个来自 PostgreSQL
			驱动的认证失败错误，那个报错不会告诉你"你的环境变量没设"。
			现在配置缺失在第一步就被拦住，报错直接说缺哪个变量。
		这个原则叫 fail fast——让错误在离原因最近的地方暴露。
	*/
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// 第1步：连接数据库
	/*
		数据库初始化已经独立到 db.go 中。
		InitDB() 负责：
			1. 准备 PostgreSQL 连接字符串
			2. 创建 *sql.DB
			3. Ping 测试数据库
			4. 成功返回 db
			5. 失败返回 error
		而这里的 main 只负责接收初始化结果。
	*/
	db, err := InitDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	// main 正常结束前关闭数据库
	defer db.Close()
	fmt.Println("✅ Go成功连接 PostgreSQL go_demo 数据库！ - main.go:38")

	/*
		目前数据库部分的执行流程：

		Go程序启动
		     ↓
		   main()
		     ↓
		调用 InitDB()
		     │
		     │
		     ▼
		┌───────────────────────────────────────────────┐
		│                    db.go                      │
		│                                               │
		│  InitDB()                                     │
		│      ↓                                        │
		│  准备 connStr                                 │
		│      ↓                                        │
		│  sql.Open("postgres", connStr)                │
		│      ↓                                        │
		│  返回 db + err                                │
		│      ↓                                        │
		│  err != nil ?                                 │
		│    ┌───────┴───────┐                          │
		│   是                否                        │
		│   ↓                  ↓                        │
		│ return nil,error   db.Ping()                  │
		│                      ↓                        │
		│                   err != nil ?                │
		│                    ┌─────┴─────┐              │
		│                   是           否             |
		│                   ↓             ↓             │
		│              db.Close()     return db,nil     │
		│                   ↓             ↓             |
		│              return nil,error                 │
		└———————————————————————————————————————————————┘
		                    │
		                    │ 返回 InitDB() 的结果
		                    │
		            ┌───────┴────────┐
		            ↓                ↓
		      InitDB()成功        InitDB()失败
		            ↓                ↓
		      db = *sql.DB       err != nil
		      err = nil              ↓
		            ↓             log.Fatal(err)
		      defer db.Close()       ↓
		            ↓              程序退出
		      打印数据库连接成功
		            ↓
		      main继续执行
		            ↓
		      创建 Gin 路由器
	*/
	/*
		注意：
		log.Fatal 会立即结束程序，这种情况下 defer 不会执行。
		所以 db.Close() 主要保证 main 正常返回时释放资源。
	*/

	// 第2步：加载路由表
	router := SetupRouter(db)

	// 第3步：创建 HTTP 服务器（替代 router.Run）
	/*
		以前用 router.Run()，它直接霸占主线程，我们拿不到控制权。
		现在我们把 Gin 装进标准库的 http.Server 里，相当于手里握住了
		服务器的“遥控器”——可以自己控制启动、停止、等待。

		Addr:    cfg.ServerAddr → 监听地址，默认 ":8080"（表示所有网卡的 8080 端口），
		                          可通过环境变量 SERVER_ADDR 覆盖
		Handler: router         → 所有 HTTP 请求交给 Gin 路由处理
	*/
	server := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: router,
	}

	// 第4步：设置“系统信号陷阱”
	/*
		核心理解：signal.NotifyContext 是“信号翻译器”
		它把操作系统信号（按 Ctrl+C 或系统发 kill 命令）翻译成
		Context 的取消信号。当信号到来时，ctx.Done() 会立刻收到通知。

		核心理解：context.Background() 是“永恒的根”
		它是 Go 里最顶层的空 Context，没有超时、没有取消，永远存在。
		用它的原因是希望这个服务器的“寿命”完全由操作系统信号控制，
		而不是被某个定时器限制。

		两个监听信号：
		  - os.Interrupt    → 按 Ctrl+C 发出的中断信号
		  - syscall.SIGTERM → 系统或云平台发来的终止信号（kill 命令）
	*/
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,    // Ctrl+C
		syscall.SIGTERM, // kill 命令
	)
	// 程序结束时停止监听信号，释放内部资源。
	defer stop()

	// 第5步：在后台 goroutine 里启动 HTTP 服务
	/*
		★核心理解：ListenAndServe 是“真正执行任务”
		server.ListenAndServe() 是真正让 Go 程序开始监听 8080 端口、
		接受 TCP 连接、并把请求交给 Gin 处理的那个底层循环。

		核心理解：if 是“嵌套判断”
		把这一行拆开看就完全清楚了：

		    err := server.ListenAndServe()  // ① 真正执行：开始监听端口
		    if err != nil && err != http.ErrServerClosed {
		        log.Fatalf(...)            // ② 如果报错且不是因为“正常关闭”，程序崩溃
		    }

		这和在 C++ 里写 if ((err = doSomething()) != 0) { ... } 是一模一样的逻辑。
		ListenAndServe() 只有在以下情况才会返回：
		  - 正常关闭（返回 http.ErrServerClosed）→ 这是预期行为，不算错误
		  - 启动失败（端口被占用等）→ 报错，程序崩溃
	*/
	go func() {
		log.Printf("HTTP 服务器启动，监听 %s - main.go:160", cfg.ServerAddr)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务器启动失败: %v", err)
		}
	}()

	// 第6步：阻塞等待退出信号（这是整个程序的“生死大门”）
	/*
		这是整个流程里最关键的步骤！

		<-ctx.Done() 会让主线程彻底卡在这里，什么都不做，直到 ctx 被取消。
		ctx 什么时候被取消？只有当你按 Ctrl+C 或系统发 kill 命令的时候。

		也就是说：
		  - 在这之前，第7步和第8步根本不存在（它们还没被执行）
		  - 只有第6步的“门”打开了，第7、8步才有机会执行
		★第6步是第7、8步的前提。

		这是整个优雅关闭机制的骨架。
	*/
	<-ctx.Done()
	log.Println("收到退出信号，开始优雅关闭服务器... - main.go:182")

	// 第7步：给正在处理的请求一段“宽限期”
	/*
		到这一步，说明第6步的“门”已经打开了，我们确定要退出了。
		但是！我们不能直接拔电源，因为可能还有 HTTP 请求正在处理中
		（比如正在写数据库、正在返回响应）。

		shutdownCtx 是一个倒计时器，时长由 cfg.ShutdownTimeout 决定
		（默认 5 秒，可通过环境变量 SHUTDOWN_TIMEOUT 覆盖，写法形如 "5s"、"30s"）：
		  - 它给了正在运行的请求最多这么长时间来完成工作
		  - 超时后如果还有请求没完成，就会被强制中断

		为什么用 context.Background()？
		  - 因为它不会被任何事情取消，只需要它来控制超时时间
	*/
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// 第8步：执行优雅关闭
	/*
		server.Shutdown() 会做三件事：
		  1. 立即关闭 8080 端口的监听 → 新请求进不来了
		  2. 等待所有正在处理中的请求自然完成
		  3. 如果超过宽限期（shutdownCtx 超时）还有请求没完成，强制中断

		注意：这一步能执行，完全依赖于第6步的 ctx.Done() 被触发。
		如果第6步不放行，程序永远不会走到这里。
	*/
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 服务器关闭失败: %v - main.go:211", err)
	}

	log.Println("HTTP 服务器已关闭 - main.go:214")

	/*
		程序退出前的最后一步：
		第1步里有 defer db.Close()。main 函数执行到这里结束，defer 被触发，数据库连接释放。
	*/
}
