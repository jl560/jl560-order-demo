# jl560-order-demo

一个用 Go + Gin + PostgreSQL 写的用户管理服务，同时是一个软件工程训练项目。

当前提供用户的增删改查接口，以及若干 Go 并发机制的实验接口。

## 技术栈

- **Go 1.26** —— 服务端语言
- **Gin** —— HTTP Web 框架
- **PostgreSQL** —— 关系型数据库
- **React + TypeScript + Vite** —— 前端。`npm run dev` 在 :5173，请求发往 :8080
- **Docker Compose** —— 另一套运行方式。开发用 `docker-compose.yml`，生产用 `docker-compose.prod.yml`

用户增删改查仍在一个 API 进程里。gRPC、Redis、Kafka 已经在仓库中，但不在这条主链上：gRPC 只做跨进程实验，Redis 只缓存 `GET /users/:id`，Kafka 只发送创建用户成功后的 `UserCreated`。生产 Compose 不部署这三项。

## 前置条件

- 已安装 Go 1.26 或更高版本（`go version` 能输出版本号）
- 已安装并启动 PostgreSQL（本项目在 PostgreSQL 17 上开发）
- 已安装 Node.js（前端 `npm run dev` 需要）

## 启动步骤

### 1. 创建数据库

用 pgAdmin 或 psql 创建一个数据库，名字随意，默认用 `go_demo`：

```sql
CREATE DATABASE go_demo;
```

### 2. 建表

执行仓库根目录的 [`schema.sql`](schema.sql)。两种方式任选：

**方式 A：pgAdmin（图形界面）**

左侧选中 `go_demo` 数据库 → 右键 → Query Tool → 打开 `schema.sql` → 执行。

**方式 B：psql 命令行**

`psql` 默认不在 Windows 的 PATH 上，所以要用完整路径：

```powershell
& "C:\Program Files\PostgreSQL\17\bin\psql.exe" -U postgres -d go_demo -f schema.sql
```

> 列从 `password` 改成了 `password_hash`。如果你已经有旧表，必须先
> `DROP TABLE IF EXISTS users;` 再执行 `schema.sql`，否则 INSERT 会对不上列名。

### 3. 配置

把配置模板复制一份，填上你自己的真实值：

```powershell
Copy-Item .env.example .env
```

然后编辑 `.env`，至少填好这三项（其余有默认值）：

```
DB_USER=postgres
DB_PASSWORD=你的数据库密码
DB_NAME=go_demo
```

> `.env` 已被 `.gitignore` 忽略，不会进入 git。
> 真实密码只存在于你本机，不要提交，也不要写进任何进 git 的文件。

### 4. 启动

需要两个进程。

终端 1 —— Go API（:8080）：

```powershell
go mod download
go run .
```

终端 2 —— React 页面（:5173）：

```powershell
cd frontend
npm install
npm run dev
```

浏览器打开 Vite 提示的地址，默认 `http://localhost:5173/`。
不要用 :8080 打开页面：Go 现在只提供 API，不再托管 HTML。

页面里的请求发往 `http://127.0.0.1:8080`，不经过 Vite 代理。

看到下面两行说明启动成功：

```
✅ Go成功连接 PostgreSQL go_demo 数据库！
HTTP 服务器启动，监听 :8080
```

按 `Ctrl+C` 退出，服务会优雅关闭（先停止接收新请求，等在途请求做完，再退出）。

### 5. 用 Docker Compose 启动

这是另一套运行方式，不替换上面的本机 PostgreSQL。先停掉占用 `5173` 和 `8080` 的本机进程，并确认仓库根目录已有 `.env`（Compose 只用里面的 `DB_USER`、`DB_PASSWORD`、`DB_NAME`）。

```powershell
docker compose up --build
```

浏览器打开 `http://localhost:5173/`。三个服务的关系：

| 服务 | 容器内端口 | 宿主机端口 | 谁来访问 |
| --- | --- | --- | --- |
| frontend | 80 | 5173 | 浏览器 |
| backend | 8080 | 8080 | frontend 的 Nginx，以及直接调用 API 的客户端 |
| postgres | 5432 | 5433 | 只有 backend 容器，走主机名 `postgres` |

Compose 里的页面是构建产物，接口基址是 `/api`。浏览器访问 `http://localhost:5173/api/...`，frontend Nginx 去掉 `/api` 前缀后转到 `backend:8080`。本机 `npm run dev` 不走这条路径，仍直连 `http://127.0.0.1:8080`。

backend 的 `DB_HOST` 在 Compose 里被设为 `postgres`。容器里的 `localhost` 是 backend 自己，不是 Windows，也不是数据库容器。宿主机上原来的 PostgreSQL 仍在 `5432`，数据目录没有被挂进容器。

数据库文件在名为 `pgdata` 的数据卷里。`docker compose down` 只停容器，卷还在。`docker compose down -v` 会删掉这个卷，Compose 里的库才会消失。

## 配置项

所有配置通过环境变量传入。本地开发写在 `.env` 里，部署时由运行环境注入。

| 变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `DB_USER` | 是 | —— | 数据库用户名 |
| `DB_PASSWORD` | 是 | —— | 数据库密码 |
| `DB_NAME` | 是 | —— | 数据库名 |
| `DB_HOST` | 否 | `localhost` | 数据库地址 |
| `DB_PORT` | 否 | `5432` | 数据库端口 |
| `DB_SSLMODE` | 否 | `disable` | SSL 模式 |
| `SERVER_ADDR` | 否 | `:8080` | HTTP 监听地址 |
| `SHUTDOWN_TIMEOUT` | 否 | `5s` | 优雅关闭的宽限期 |
| `REDIS_ADDR` | 否 | 空 | 空则不连接 Redis，`GET /users/:id` 直接查 PostgreSQL |
| `KAFKA_ADDR` | 否 | 空 | 空则只把 `UserCreated` 留在 outbox，不启动 Publisher |

必填项没有默认值，是因为默认值要么是秘密（密码），要么在每台机器上本来就不同。
缺任何一项，程序会在启动时立即退出并告诉你缺哪个，而不是等到第一个请求才报错。

## API

| 方法 | 路径 | 说明 | 成功状态码 |
| --- | --- | --- | --- |
| GET | `/ping` | 存活探测 | 200 |
| POST | `/users` | 创建用户 | 201 |
| GET | `/users` | 查询全部用户（不含密码） | 200 |
| GET | `/users/:id` | 查询单个用户（不含密码） | 200 |
| PUT | `/users/:id` | 更新用户 | 200 |
| DELETE | `/users/:id` | 删除用户 | 200 |

请求体字段（POST / PUT）：

- `username`：必填，3–32 个字符
- `password`：必填，至少 6 个字符（只用于写入，不会出现在 GET 响应里）
- `age`：0–150
- 请求里的 `id` 会被忽略；主键由数据库自增，更新时 id 走 URL

请求示例：

```powershell
# 创建（密码至少 6 位）
curl.exe -X POST http://localhost:8080/users -H "Content-Type: application/json" -d '{\"username\":\"alice\",\"password\":\"123456\",\"age\":20}'

# 查询全部 —— 响应里只有 id、username、age，没有 password
curl.exe http://localhost:8080/users
```

空 body 或字段不合法会返回 400。

另有四个 Go 并发机制的实验接口：`/test-timeout`、`/test-goroutine`、
`/test-waitgroup`、`/test-channel`。它们是学习用的，不属于业务功能。

## 项目结构

```
main.go            进程生命周期：加载配置 → 连库 → 建路由 → 启动 → 优雅关闭
config.go          配置加载：从环境变量读出 Config，缺必填项则拒绝启动
db.go              数据库初始化：拼连接串、建连接池、Ping 验证
router.go          路由注册与 HTTP 处理
user.go            请求体 / 数据库行 / 响应体三种类型
user_service.go    用户业务：bcrypt；创建用户时和 outbox 同一事务
user_repository.go 用户表的 SQL
user_cache.go      只缓存 GET /users/:id
outbox_repository.go  outbox 的 SQL
outbox_publisher.go   把未发送事件送到 Kafka
schema.sql         users 和 outbox 的建表语句
Dockerfile         Go API 镜像
docker-compose.yml 开发 Compose：5173、8080、5433
docker-compose.prod.yml  生产 Compose：页面和 /api 走 frontend
cmd/grpcserver     实验用 gRPC，不参与用户 CRUD
cmd/userconsumer   打印 UserCreated，不参与用户 CRUD
proto/  gen/       gRPC 实验的协议和生成代码
frontend/          React + TypeScript + Vite
                   src/types  API JSON 类型
                   src/api    只负责 HTTP
                   src/components  只负责 UI
.env.example       配置模板（不含真实值）
```

## 当前阶段

用户请求的主链是：

```text
Browser → React → HTTP/JSON → Gin Handler → Service → Repository → PostgreSQL
```

| 阶段 | 已完成的内容 |
| --- | --- |
| F6 | 开发页面跨源调用 API |
| F7 | Handler、Service、Repository |
| F8 | 区分 409 和 500，并补测试 |
| F9 | 开发用 Docker Compose |
| F10 | 实验用 gRPC：`InspectUsername` 和 `GET /test-grpc` |
| F11 | Redis Cache Aside，只缓存 `GET /users/:id` |
| F12 | 创建用户和 outbox 同一事务；Kafka 发送 `UserCreated` |
| F13A | 生产构建走 frontend Nginx 的 `/api` |

生产启动：

```powershell
docker compose -f docker-compose.prod.yml up -d --build
```

项目名是 `jl560-order-demo-prod`。frontend 只绑定 `127.0.0.1:8088:80`。backend 和 postgres 没有宿主机端口。这个 Compose 不包含 Redis、Kafka、gRPC。`KAFKA_ADDR` 为空时，创建用户仍会留下尚未发送的 outbox 行。

没有登录。公网 IP 验收还没做。
