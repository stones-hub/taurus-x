# AGENTS.md

> 本项目长期稳定的研发、验收约束。当前任务和进度写在 `.pi/tasks/` 与 `HANDOVER.md`。

## 技术栈与主要入口

- 语言/框架：Go 1.25（CLI 本身）；生成的项目为 Go（Taurus Pro 组件 + GORM + Wire）+ Vue3/TS/Vite/Element Plus/Pinia
- 自身 module：`github.com/stones-hub/taurus-x`
- 主要入口：`main.go` → `taurus-x new <项目名>`（交互式）、`taurus-x version`
- 模板：`templates/`，通过 `embed` 打进二进制；只有 `.tmpl` 后缀文件走 Go 模板渲染，其余原样复制
- 模板来源：`/Users/yelei/data/code/repos/admin`（只读参考，禁止修改该仓库）
- 正式交付形态：CLI 二进制（`go install github.com/stones-hub/taurus-x@latest`）
- 已确认方案：`docs/方案.md`

## 代码、构建和测试

- 构建：`go build -o /tmp/taurusx-bin/taurus-x .`（产物不放仓库根）
- 测试：`go vet ./... && go test ./...`
- 生成器测试必须覆盖：只选必选组件、全选、典型组合；生成后在临时目录执行 `go build ./...` 通过
- Go 缓存权限问题时使用 `GOCACHE=/tmp/gocache-taurusx`
- 前端构建验证：本机未全局安装 pnpm，使用 `npm exec --yes pnpm@10.34.7 -- <cmd>`，不全局安装
- 数据库：MySQL 8.0 + Redis 7（本机验收用 Docker 测试容器，禁止用 SQLite 冒充）

## 本机候选运行验收

- 方式：构建 taurus-x → 在 `/tmp/taurusx-e2e/` 生成测试项目 → Docker 起测试 MySQL/Redis（独立容器名 `taurusx-e2e-*`、独立端口）→ 导入生成项目的 `scripts/sql/init.sql` → 构建前端与后端 → 原生启动后端，仅监听 `127.0.0.1`
- 业务就绪：`/health` 返回正常，`/login` 返回页面
- 黑盒场景：独立测试浏览器登录初始管理员，打开用户/角色/权限/部门/各日志/个人中心页面，控制台无 error
- 日志：生成项目 `backend/logs/`
- 清理：停止测试进程，删除 `taurusx-e2e-*` 容器与卷，删除 `/tmp/taurusx-*`
- 不得占用或停止 admin 项目正在运行的容器（`admin-local-mysql`、`admin-local-redis`）和 9080 端口进程

## 保护路径与禁止操作

- 禁止修改 `/Users/yelei/data/code/repos/admin` 及 Go 模块缓存
- 禁止读取或输出凭证、`.env`、`.env.local`、`*account*.txt` 等敏感内容
- 禁止未经授权的 `git commit`、`git push`、merge、rebase、`reset --hard`、checkout/restore 清理未提交改动
- 禁止全局安装软件（pnpm、wire 等），禁止修改系统配置
- 禁止为过测试而删除、跳过或放松测试和断言
- 越界后不得自行破坏性回滚；先报告并等待人工确认

## 执行器与授权

- 默认编码执行器为 Claude Code；执行器失败或结果不明确时停止，不静默切换或重试
- 同一工作区同一时刻只运行一个有写权限的编码 Agent
- 方案确认不等于编码授权；commit、push、发布分别授权

## 项目完成标准

- `go vet ./... && go test ./...` 通过
- 全选组件生成的项目：后端 `go build`、前端 `pnpm build` 通过，本机运行后浏览器登录及底座页面验收通过
- 涉及认证/权限/安全的改动为关键任务，需至少一路独立复审
