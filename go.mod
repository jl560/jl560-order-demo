// go.mod：Go模块配置文件，属于Go语言，不属于Git
// 1、由 go mod init 命令生成；一个项目只在一开始执行一次init
// 2、记录项目模块名、要求的Go版本、第三方依赖包及版本
// 3、go get新增依赖时会自动追加require条目；手写//注释会被保留
// 4、不会存储第三方包源码，仅仅做记录；他人克隆项目靠go mod download下载依赖

// module：定义当前Go项目模块名称，第三方引用本项目时使用该标识
module jl560-order-demo

// go：声明本项目编译运行要求的最低Go语言版本
go 1.26.5

// require：声明项目依赖的第三方包以及锁定版本
// 分成两类：
//   没有 //indirect 标记 = 直接依赖，本项目代码里有 import 它
//   有   //indirect 标记 = 间接依赖，本项目没直接用，是别的包引入的
// go mod tidy 会扫描所有 .go 文件的 import，自动维护这些标记
//
// github.com/lib/pq：PostgreSQL数据库驱动（db.go 匿名导入）
require github.com/lib/pq v1.12.3

require (
	github.com/gin-gonic/gin v1.12.0
	github.com/joho/godotenv v1.5.1
	golang.org/x/crypto v0.57.0
)

require (
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.15.0 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.30.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.59.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.5.0 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)
