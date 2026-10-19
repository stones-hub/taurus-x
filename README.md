# taurus-x

Go + Vue 后台底座脚手架。一条命令生成一个**只含管理后台最小闭环**的全新项目：
登录鉴权、用户 / 角色 / 权限（菜单 / 按钮 / 接口三级 RBAC）、部门与部门成员、
四类日志（登录 / 访问 / 操作 / 前端错误），**不含任何业务模块**。

生成的项目后端 Go + Google Wire + GORM，前端 Vue3 + TypeScript + Vite + Element Plus，
前端构建产物由 Go 进程直接托管（SPA history 回退），单端口对外。

## 安装与使用

```bash
go build -o taurus-x .          # 或 go install github.com/stones-hub/taurus-x@latest
./taurus-x new my-admin
```

`new` 会依次问四件事，然后生成项目：

1. **Go module 路径**（默认 `github.com/stones-hub/<项目名>`）
2. **输出目录**（默认 `./<项目名>`；目录必须不存在或为空，**不会覆盖已有内容**）
3. **组件多选**（空格勾选 / 回车确认）
4. **汇总确认**

生成后自动执行 `go mod tidy` 与 Wire 代码生成，并打印下一步（建库导 SQL、配 `.env.local`、
装前端依赖、`make dev`）和初始管理员账号。

其它命令：`taurus-x version`、`taurus-x help`。

## 组件

| 组件 | 必选 | 作用 |
|------|------|------|
| common | ✅ | 日志 / JWT / 工具库 |
| config | ✅ | 配置加载与环境变量替换 |
| http | ✅ | HTTP 服务与中间件 |
| storage | ✅ | MySQL + Redis |
| consul | | 服务注册发现 |
| opentelemetry | | 链路追踪 / 指标上报 |
| tcp | | TCP 长连接服务 |
| grpc | | gRPC 服务 |
| milvus | | 向量库客户端池 |

必选组件取消勾选也会被补回（底座依赖它们）。**未选中的可选组件在生成的项目里完全不存在**：
不在 `go.mod`、不在 `internal/taurus`（`Components` 字段 / `Provide*` / `wire.Build`）、
不在 `config/autoload/`、不在 `docker-compose.yml`、不在 `Dockerfile` 的 `EXPOSE`。

## 生成的项目

初始管理员账号为 `admin`，默认密码写在生成项目的 `README.md` 里；密码以
`SHA-256(密码 + salt)` 的小写 hex 存入 `admin_user_logins`，**salt 与 JWT 密钥每次生成都随机**。
建表与权限数据只走 `scripts/sql/init.sql`（**不开 GORM AutoMigrate**），其中含 12 张底座表
与 74 条权限（12 菜单 + 21 按钮 + 41 接口）。

> ⚠️ 生成项目的 `README.md`「已知风险与待办」章节列出了底座仍待修复的问题
> （SHA-256 强度、JWT 弱默认值、日志未脱敏、`dept_code` 唯一索引缺陷等），**上线前先读**。

## 本仓库结构

```
taurus-x/
├── main.go
├── internal/
│   ├── cli/              # 命令行入口与交互（survey）
│   ├── components/       # 组件注册表（名称 / 包路径 / 版本 / 是否必选）
│   └── generator/        # 渲染逻辑 + 组合测试
└── templates/
    ├── templates.go      # go:embed all:project
    └── project/          # 项目模板树
```

模板约定：**只有 `.tmpl` 后缀的文件走 `text/template` 渲染，其余原样复制**。
Vue/TS/CSS 里的 `{{ }}` 是 Vue 插值，这些文件一律不加 `.tmpl`；而所有 `.go`、`go.mod`
必须加 `.tmpl`（它们要替换 module 路径）。`gitignore` 在模板里不带点，输出时由
`renameOnOutput` 还原成 `.gitignore`（否则会干扰本仓库自身）。

## 开发

```bash
go vet ./...
go test ./...     # 三种组件组合各跑一遍：生成 → go mod tidy → wire → go build → go vet
```

组合测试产物写在 `/tmp/taurusx-test-*`，不落仓库。测试同时断言：
无业务残留（CRM / 订单 / 财务 / 资源 / 任务 / 地点关键词与 `excelize` / `gomail` 依赖）、
无凭证文件与内网标识串、未选组件不泄漏、`init.sql` 的表与接口权限 path 齐全、
模板占位全部渲染完。

前端构建需手动验证（测试里不跑，避免拉依赖）：

```bash
cd /tmp/taurusx-test-all/frontend
pnpm install && pnpm build
```
