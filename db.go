package main

import (
	"database/sql" // 提供访问关系型数据库的通用接口
	"fmt"          // 提供格式化输入输出与错误包装

	// 第三方包
	/*
		pq 是第三方 PostgreSQL 驱动，不属于 Go 标准库。

		database/sql 提供的是"访问数据库的通用抽象层"（定义接口），
		而 pq 负责实现这些接口背后的"具体细节"：
			建立 TCP 连接、完成 PostgreSQL 认证握手、
			按 PostgreSQL 协议编码 SQL 并发送、解码返回的数据包。

		之所以在导入时加下划线 _（匿名导入），是因为：
		我们不直接在代码里调用 pq 包的任何函数或方法，
		而是需要它在程序启动时"主动完成注册"，把驱动信息告诉 database/sql。

		具体执行流程如下：
		1. Go 程序启动时，加载所有 import 的包，并执行各包的 init() 函数。
		2. pq 包的 init() 内部会调用 sql.Register("postgres", &pq.Driver{})，
			将自己注册到 database/sql 的全局驱动映射表里。
		3. 注册完成后，后续代码执行 sql.Open("postgres", connStr) 时，
			database/sql 才能根据字符串 "postgres" 找到对应的驱动实现，
			进而创建出可以操作 PostgreSQL 的 *sql.DB 对象。

		注意：依赖管理工具（如 go mod）会在编译前下载该包到本地缓存，
		这与程序运行时的注册机制是两回事，不需要在注释中混为一谈。
	*/
	_ "github.com/lib/pq"
)

/*
数据库初始化流程：

第 1 步：准备数据库连接字符串（connection string）

connStr 是变量名，存储了连接 PostgreSQL 所需的所有参数：

	host     : 数据库服务器地址（localhost 表示本机）
	port     : 监听端口（5432 是 PostgreSQL 默认端口）
	user     : 登录用户名
	password : 登录密码
	dbname   : 要连接的数据库名称（go_demo）
	sslmode  : SSL 加密模式（disable 表示关闭，开发环境常用）

这些值不再硬编码在本文件里，而是由调用方通过 cfg 参数传进来。
cfg 的内容最终来自环境变量（见 config.go）。这样做的好处：
同一份编译产物可以连不同环境的数据库，而且密码不进源码。

第 2 步：调用 sql.Open() 创建数据库操作入口

sql.Open("postgres", connStr) 的各个部分：

	sql        : database/sql 标准库包名
	Open       : 该包提供的函数，用于初始化数据库连接管理器
	"postgres" : 第一个参数，指定使用哪个已注册的数据库驱动
	connStr    : 第二个参数，传入上一步准备好的连接字符串

多返回值（Go 常见模式）：

	sql.Open 返回两个值：
		第一个返回值 (*sql.DB) → 赋值给变量 db
		第二个返回值 (error)   → 赋值给变量 err

这种模式在 Go 中非常常见：正常结果 + error。

db 的本质是什么？

	db 的类型是 *sql.DB，即“指向 sql.DB 结构体的指针”。
	db 变量里保存的是一个指向 sql.DB 对象的地址。

	sql.DB 本身可以理解成一个数据库操作管理对象，
	内部负责管理数据库连接、连接池以及相关状态。

	因此：

	    db（变量）
	        ↓
	    *sql.DB（指针）
	        ↓
	    sql.DB 对象
	        ↓
	    管理底层 PostgreSQL 数据库连接

【db 的内存模型】

db（变量）

	│
	│ 是一个指针，保存了一个内存地址
	▼

┌─────────────────────────────────────────────────────┐
│                    sql.DB 对象                      │
│  ┌───────────────────────────────────────────────┐  │
│  │  连接池管理                                    │  │
│  │  ┌─────────────┐  ┌─────────────┐             │  │
│  │  │  空闲连接    │  │  在用连接   │             │  │
│  │  │  (idle)     │  │  (in use)   │             │  │
│  │  └─────────────┘  └─────────────┘             │  │
│  ├───────────────────────────────────────────────┤  │
│  │  配置参数：                                    │  │
│  │  - 最大打开连接数 (MaxOpenConns)               │  │
│  │  - 最大空闲连接数 (MaxIdleConns)               │  │
│  │  - 连接最大存活时间 (ConnMaxLifetime)          │  │
│  ├───────────────────────────────────────────────┤  │
│  │  并发控制：互斥锁 (Mutex)                      │  │
│  ├───────────────────────────────────────────────┤  │
│  │  驱动信息：指向已注册的 PostgreSQL 驱动         │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘

			│
			│ 通过数据库驱动与 PostgreSQL 通信
			▼
	┌───────────────┐
	│  PostgreSQL   │
	│ 数据库服务器   │
	└───────────────┘

【关键理解】

db 不是数据库本身，而是一个“数据库操作入口对象”的指针。
通过 db 调用方法（如 db.QueryContext()），
实际上是让 sql.DB 对象利用其内部管理的连接去和真实的 PostgreSQL 通信。

Go 中对指针和值类型的方法调用都使用 . 语法。
例如这里：

	db.Ping()

可以理解为：
“通过 db 找到它指向的 sql.DB 对象，
然后调用这个对象的 Ping 方法。”

重要：
sql.Open() 此时不等于已经成功连接数据库。

它主要负责创建数据库操作管理对象并关联指定驱动。
真正的数据库通信可以发生在：

	db.Ping()
	db.ExecContext()
	db.QueryContext()

等操作中。

第 3 步：Ping 数据库

db.Ping() 用来实际测试程序能否正常访问 PostgreSQL。
如果 Ping 失败，则说明数据库目前无法正常访问。

第 4 步：资源释放

如果 db.Ping() 失败，InitDB() 主动调用 db.Close()，
释放已经创建的数据库操作对象相关资源。

如果 Ping 成功，则由 main() 中的 defer db.Close()
负责在服务最终退出时释放数据库资源。
*/
func InitDB(cfg *Config) (*sql.DB, error) {
	/*
		密码用单引号包起来的原因：
		这种 keyword=value 格式以空格分隔各个字段，密码里一旦含有空格，
		不加引号就会被截断成两个字段。报出来的错误会是"参数无法识别"之类，
		完全不指向真正的原因，属于极难排查的一类坑。
	*/
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password='%s' dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Ping数据库失败: %w", err)
	}

	return db, nil
}
