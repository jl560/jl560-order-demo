package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	// 第三方包
	/*
		godotenv 的作用只有一件事：把 .env 文件里的键值对，
		塞进当前进程的环境变量表。

		为什么需要它？
			环境变量是操作系统交给进程的一张键值表，它的生命周期跟着进程走，
			关掉终端就没了。本地开发需要好几个变量，不想每开一个新终端
			就手打五行 $env:XXX=，所以把它们写进一个 .env 文件持久化下来。

		关键点：.env 文件本身不是环境变量，它只是一个普通文本文件。
		操作系统不认识它，Go 也不会自动读它，必须有人把它加载进来——
		在本地开发里这个"有人"就是 godotenv。
	*/
	"github.com/joho/godotenv"
)

/*
Config 保存程序运行需要的全部配置。

为什么要有这个结构体？

	配置的唯一来源是环境变量。如果让 db.go、main.go 各自去调 os.Getenv，
	"这个程序到底需要哪些配置"就散落在多个文件里，谁都说不全。
	集中成一个结构体 + 一个加载函数之后，这个问题有了唯一答案。

为什么配置要从环境读，而不是写死在代码里？

	1. 同一份程序要在不同环境跑。本机数据库在 localhost，上了 Docker
	   之后在容器服务名上，真上线了又在别的地址。写死在代码里，
	   这三种情况就是三份代码。
	2. 秘密不该和代码同命运。代码要进 git、要给别人看；密码不该。
	   放在两个地方，才有可能分别对待。
	3. 改配置不该需要改代码。改代码意味着重新编译、测试、提交；
	   改环境变量意味着重启一下。
*/
type Config struct {
	// 数据库连接参数
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// HTTP 服务参数
	ServerAddr      string        // 监听地址，形如 ":8080"
	ShutdownTimeout time.Duration // 优雅关闭的宽限期
}

/*
LoadConfig 加载配置：先把 .env 读进环境变量，再从环境变量读进 Config。

【必填与可选的划分依据】

	一个配置能有默认值，前提是这个默认值既不是秘密、又在多数情况下正确。
	  - DBHost / DBPort / DBSSLMode / ServerAddr / ShutdownTimeout 满足 → 给默认值
	  - DBUser / DBPassword / DBName 不满足 → 必填，缺了就拒绝启动
	给密码设默认值，等于又在代码里留了一个密码。

【为什么缺失项一次全报，而不是遇到第一个就返回】

	如果三个变量都没设，逐个返回会让使用者启动三次才知道全部问题。
	一次报全，改一次就能对。

【fail fast：为什么配置错误要在启动时就崩】

	改之前，配错密码的表现是：程序正常启动 → 走到 db.Ping() → 收到一个
	来自 PostgreSQL 驱动的认证失败错误。那个报错不会告诉你"你的环境变量没设"。
	现在配置缺失在第一步就被拦住，报错直接说缺哪个变量。
	原则：让错误在离原因最近的地方暴露，而不是在离原因最远的地方表现成症状。
*/
func LoadConfig() (*Config, error) {
	/*
		读取 .env 文件。这里故意忽略返回的错误，原因有两个：

		1. .env 不存在是完全正常的情况。到了 Phase 7 跑在 Docker 里时，
		   配置由容器平台直接注入环境变量，根本不会有 .env 文件。
		2. godotenv.Load() 不会覆盖已经存在的真实环境变量。
		   也就是说"外部注入"的优先级天然高于"文件里写的"，
		   这正是我们想要的行为：本地靠文件省事，线上靠环境变量权威。

		所以这一行的语义是"如果有 .env 就顺便读一下"，而不是"必须有 .env"。
	*/
	_ = godotenv.Load()

	// missing 收集所有缺失的必填项，最后一次性报出
	var missing []string

	// require：必填项。取不到就记一笔账，继续往下走（不提前返回）
	require := func(key string) string {
		value := os.Getenv(key)
		if value == "" {
			missing = append(missing, key)
		}
		return value
	}

	cfg := &Config{
		DBHost:     optional("DB_HOST", "localhost"),
		DBPort:     optional("DB_PORT", "5432"),
		DBUser:     require("DB_USER"),
		DBPassword: require("DB_PASSWORD"),
		DBName:     require("DB_NAME"),
		DBSSLMode:  optional("DB_SSLMODE", "disable"),
		ServerAddr: optional("SERVER_ADDR", ":8080"),
	}

	// 必填项的缺失比格式错误更重要，所以先报它
	if len(missing) > 0 {
		return nil, fmt.Errorf(
			"缺少必填环境变量: %s（把 .env.example 复制成 .env 并填好，详见 README.md）",
			strings.Join(missing, ", "),
		)
	}

	/*
		ShutdownTimeout 是唯一需要"解析"的配置。
		环境变量的值永远是字符串，而这里需要一个 time.Duration，
		所以用 time.ParseDuration 把 "5s" 这样的写法转成时长。
		格式写错（比如写成 "5秒"）会在这里被拦下来，同样是 fail fast。
	*/
	timeoutText := optional("SHUTDOWN_TIMEOUT", "5s")
	timeout, err := time.ParseDuration(timeoutText)
	if err != nil {
		return nil, fmt.Errorf("SHUTDOWN_TIMEOUT 格式错误（应形如 5s、30s、1m），当前值 %q: %w", timeoutText, err)
	}
	cfg.ShutdownTimeout = timeout

	return cfg, nil
}

// optional 读取环境变量，未设置或为空时返回默认值。
func optional(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
