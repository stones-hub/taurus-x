# 任务目标

T1：实现 taurus-x CLI 第一版，`taurus-x new <项目名>` 交互式选择组件后，生成一个以 admin 为蓝本、只含基础后台底座的 Go + Vue 项目，生成后能直接 `go build`，前端能 `pnpm build`，配上 MySQL/Redis 后能登录。

# Risk

- 类型：普通任务（新建工具，不涉及生产数据；底座安全加固放在 T2 关键任务中）
- 判断理由：产物为本地代码生成，最坏后果为生成的项目无法构建/运行，可重新生成
- 关键契约与升级触发条件：如果发现必须修改 admin 仓库、必须全局安装软件、或 docs/方案.md 的某项决定无法实现，立即停止并报告

# Scope

- 当前基线：taurus-x 为空仓库（已 git init，无任何 commit），仅有 `AGENTS.md`、`CLAUDE.md`、`HANDOVER.md`、`.gitignore`、`docs/方案.md`、`docs/安全审计.md`、`.pi/tasks/T1-scaffold.md`
- 允许改动：`/Users/yelei/data/code/repos/taurus-x/` 下除 `AGENTS.md`、`CLAUDE.md`、`HANDOVER.md`、`docs/方案.md`、`docs/安全审计.md`、`.pi/` 之外的所有文件；可新增 `README.md`；测试生成的临时项目只能放在 `/tmp/taurusx-*` 下
- 禁止改动：`/Users/yelei/data/code/repos/admin`（只读参考）、`/tmp/taurusx-ref/taurus-pro-core`（只读参考）、Go 模块缓存、全局环境

# 执行器与授权

- 执行器：Claude Code
- 请求模型：opus
- 编码授权：已获得（磊哥确认方案并授权编码；不含 commit/push）
- 权限范围：读写 taurus-x 仓库；读 admin 与 core 参考代码；可联网下载 Go 模块与 npm 包到各自缓存；禁止全局安装

# 真实业务背景

磊哥要一个脚手架，以后新建 Go+Vue 后台项目时一条命令生成完整骨架。完整方案见 `docs/方案.md`，必须按其中"已确认的决定"实现。

# 已确认的源码事实

- 参考项目 `/Users/yelei/data/code/repos/admin`：
  - 后端 `backend/`，module `github.com/stones-hub/taurus-admin`，入口 `bin/taurus.go`，`app/bootstrap.go` 在 `init()` 中构建组件与应用注入器
  - 框架组件容器 `backend/internal/taurus/{bootstrap.go,wire.go,wire_gen.go,daily_log.go}`：`Components{Config, DbList, Redis, Http, Cron, Logger}`，`ProvideXxxComponent` 风格
  - 应用注入：`backend/internal/wire/`（scanner 扫描 app 下 provider set 生成 `app/wire.go`），`backend/Makefile` 的 `wire` 目标先编译运行 `internal/wire/gen_wire.go` 再执行全局 `wire`
  - 路由 `backend/app/routes/routes.go`：含 `/health`、`spaHandler`（`frontendDistDir = "../frontend/dist"`，SPA 回退 + `..` 过滤）
  - 前端 `frontend/`：Vue3 + TS + Vite + Element Plus + Pinia；底座相关页面 `views/{login,home,user,role,permission,dept,log,profile}`，组件 `PermTree.vue`、`SidebarMenu.vue` 等，接口 `api/{auth,user,user-manage,role,permission,dept,log,login-log,http,option}.ts`（以实际引用为准）
  - 根目录 `Makefile`、`Dockerfile`（三阶段：node 构建前端 → go 构建后端 → alpine 运行，目录还原为 `/app/backend` + `/app/frontend/dist`）、`docker-compose.yml`、`.dockerignore`、`docs/{项目规范,后端开发流程,前端开发流程,部署文档}.md`
  - `scripts/sql/admin_init.sql`：前 12 张 `admin_*` 表（`admin_access_logs`、`admin_depts`、`admin_front_error_logs`、`admin_operation_logs`、`admin_permissions`、`admin_role_permissions`、`admin_roles`、`admin_user_depts`、`admin_user_login_logs`、`admin_user_logins`、`admin_user_roles`、`admin_users`）为底座；其余为业务表；`admin_permissions` 含业务菜单数据，需只保留底座菜单/按钮
  - 业务 helper 需去掉：`receivable_invoice.go`、`send_invoice_email.go`、`libreoffice.go`、`upload.go`、`json_file.go`（以实际引用为准）及 excelize、gomail、golang.org/x/image 依赖；`assets/` 业务文件不带
  - 不要复制 admin 中的 `.env.local`、`backend/build/`、`backend/logs/`、`node_modules`、`dist`、`HANDOVER.md`、`docs/account.md`，禁止读取 `.env.local` 和 `backend/build/local-test-account.txt`
- 参考项目 `/tmp/taurusx-ref/taurus-pro-core`（磊哥已有的旧脚手架，只读）：
  - 交互用 `github.com/AlecAivazis/survey/v2`，命令框架 cobra
  - 可选组件初始化代码：`pkg/components/{consul,grpc,otel,tcp,tmilvus}/*.go`，配置样例 `templates/config/autoload/{consul,gRPC,otel,tcp,milvus}/`
- 组件版本（本机 `/Users/yelei/data/code/go-path/pkg/mod/github.com/stones-hub/` 已有）：common v0.2.16（admin 用 v0.2.15，选其一并说明）、config v0.0.4、http v0.1.6、storage v0.1.35、consul v0.1.2、grpc v0.0.2、opentelemetry v0.0.2、tcp v0.0.2、milvus v0.0.7
- 本机：go1.25.14 darwin/arm64；全局有 `wire`，**没有 pnpm**，用 `npm exec --yes pnpm@10.34.7 -- <cmd>`；node 在 `/opt/homebrew/bin/node`

# 预期正确行为

- 正常路径：
  1. `taurus-x new myproj` 交互：module 路径（默认 `github.com/stones-hub/<项目名>`）→ 输出目录（默认 `./<项目名>`）→ 组件多选（4 必选显示为已选且不可取消；5 可选默认不选）→ 汇总确认
  2. 生成 `docs/方案.md` 第 4 节结构的项目；按项目名/module 替换 import、二进制名、`app_name`、默认库名、容器名、前端标题、文档中的项目名
  3. `internal/taurus` 的 `Components`、`Provide*`、`wire.Build`、`config/autoload/*`、go.mod require、docker-compose 服务、Dockerfile EXPOSE 都只包含所选组件；tcp/grpc 服务与 HTTP 一起启动并优雅关闭；consul 注册/注销；otel 关闭时 flush；milvus 创建客户端；每个可选组件配置带 `enable` 开关，未启用时返回 nil 且不影响启动
  4. 生成后自动执行 `go mod tidy` 与 Wire 生成（scanner + wire），不依赖全局 `wire` 命令；生成项目自己的 `backend/Makefile` 的 `wire` 目标同样不依赖全局 wire
  5. 打印下一步说明：建库并导入 `scripts/sql/init.sql`、复制 `.env.example` 为 `.env.local` 并填写、`pnpm install`、`make dev`，以及初始管理员账号/默认密码（写在生成项目 README，提示登录后立即修改）
  6. `taurus-x version` 输出版本
- 关键异常/边界：
  - 项目名非法（空、含路径分隔符或非 `[a-zA-Z0-9_-]`）报错退出
  - 输出目录已存在且非空：报错退出，不覆盖
  - tidy/wire 失败：明确报错，保留已生成文件，不自动删除
  - 用户在交互中取消（Ctrl+C）：退出且不留半成品目录
- T1 内随裁剪直接删除（见 `docs/安全审计.md`"处理分工"）：`/downloads/` 路由与 `helper/upload.go`、`/auth/register` 路由（后端 controller/service 中仅为注册服务的代码一并删除）、无人使用的 `AuthMiddleware`、`http.yaml` 中 `secret` 与 `authorization` 两行末尾多余的 `"`。T2 范围内的其他安全问题本任务不改。

# 实现约束

- 模板放 `templates/`，用 `//go:embed all:templates`（必须带 `all:` 才能包含 `.gitignore`、`.env.example`、`.dockerignore` 等点文件）
- 模板中所有 Go 源文件和 `go.mod` 必须用 `.tmpl` 后缀（如 `main.go.tmpl`、`go.mod.tmpl`），否则会被当作 taurus-x 自身的包编译，`go.mod` 还会让 embed 排除该目录
- 只有 `.tmpl` 文件走 `text/template` 渲染，其余原样复制；Vue/TS/CSS 文件不得经过模板渲染（避免 `{{ }}` 冲突）
- 生成逻辑做成可直接调用的函数（输入：项目名、module、输出目录、组件列表、是否执行 tidy/wire），交互层只负责收集输入
- 不新增 CRUD 模块生成器、不新增 `--with` 等组件参数（方案明确只做交互式）
- 生成项目的 README 要有"已知风险"章节，列出 `docs/安全审计.md` 中 T2 将修复前仍存在的问题（T2 完成后再更新）

# Verify：自动化测试

- 编码 Agent 必须运行：`go vet ./... && go test ./...`；测试中调用生成函数，在 `/tmp/taurusx-test-*` 生成"仅必选"、"全选"、"必选+grpc+otel" 三种组合，并在生成目录执行 tidy/wire 后 `go build ./...` 与 `go vet ./...` 通过（允许用 build tag 或 `-short` 区分慢测试，但必须实际跑过并报告结果）
- 编码 Agent 还需对"全选"项目执行前端 `npm exec --yes pnpm@10.34.7 -- install` 与 `-- build` 并报告结果
- Pi 必须独立重跑：上述全部

# Verify：本机候选运行验收（由 Pi 执行，编码 Agent 无需启动服务或 Docker）

- 是否改变运行行为：是
- 运行方式：构建 taurus-x 二进制 → 生成"全选"测试项目 → Docker 测试 MySQL 8.0 / Redis 7（容器名 `taurusx-e2e-*`，独立端口）→ 原生启动生成项目后端（127.0.0.1）
- 业务就绪：`/health` 正常；`/login` 返回页面
- 黑盒场景：独立测试浏览器登录初始管理员，打开用户/角色/权限/部门/各日志/个人中心，控制台无 error
- 交互流程：真实终端走一遍 `taurus-x new`（含已存在目录报错）
- 必须检查的日志：生成项目 `backend/logs/` 与启动 stdout 无未解释错误
- 停止和清理：停进程，删 `taurusx-e2e-*` 容器与卷，删 `/tmp/taurusx-*`
- 禁止连接：admin 正在运行的 `admin-local-mysql`、`admin-local-redis`、9080 端口进程；任何远程环境

# 验收标准

- `go vet ./... && go test ./...` 通过，三种组合生成后 `go build` 通过
- 生成项目不含任何业务模块代码、业务表、业务菜单、excelize/gomail 依赖；`grep -ri "taurus-admin"` 在生成项目中无残留
- 未选组件不出现在生成项目的 go.mod、配置、`internal/taurus`、compose 中
- 全选项目前端 `pnpm build` 通过；本机运行与浏览器登录验收通过
- 编码 Agent 自报、单元测试、进程/容器存活均不能单独构成完成

# 禁止事项

- 禁止读取或输出凭证（admin 的 `.env.local`、`backend/build/local-test-account.txt` 等）
- 禁止 commit、push；禁止修改 admin 仓库与 core 参考目录；禁止全局安装软件
- 禁止启动 Docker 容器或长期运行的服务（留给 Pi 验收）
- 遇到范围冲突、前提错误或验证受阻时停止并报告，不静默改方案或降低标准

# 完成后请在最终回复中报告

1. 新增/修改的文件结构概览
2. 组件版本选择及理由
3. 初始管理员账号与默认密码的设计（密码散列如何生成）
4. 三种组合测试与前端构建的实际命令和结果
5. 与方案不一致或未完成的地方

# 窄修 1（Pi 独立验收发现，续接原会话执行）

Pi 验收证据：全选项目 `.env.local` 设 `TCP_ENABLE=true`、`GRPC_ENABLE=true` 启动后，只有 HTTP 与 TCP 端口在监听，gRPC 端口未监听；日志无 "TCP server started"、无 "Waiting for signals"；SIGTERM 后进程直接退出，没有任何优雅关闭日志。关闭 TCP 后 gRPC 正常监听且 SIGTERM 优雅关闭完整。原因：`taurus-pro-tcp@v0.0.2/pkg/tcp/server.go:161-190` 的 `Start()` 在 `go s.acceptLoop()` 之后 `s.wg.Wait()` 阻塞到服务停止，而模板 `backend/app/bootstrap.go.tmpl` 在主流程同步调用。

必须修复：
1. `bootstrap.go.tmpl`：TCP 与 gRPC 一样在协程中启动，启动错误经 errChan 触发整体关闭；确保开 TCP 时 gRPC 能启动、信号监听注册、SIGTERM 优雅关闭（TCP 的关闭仍由其 cleanup 负责，确认 cleanup 能让 Start 返回）。
2. `internal/generator/generator_test.go:274-277` 含真实主机/IP/口令片段，必须删除；仓库任何文件都不得出现真实主机、IP、账号、口令或其片段。凭证防回归改为通用规则（例如：生成项目中不得存在 `docs/account*` 或文件名含 account/credential/secret 的文件），不得引用具体值。
3. 删除 `generator.go` 中 `goEnv()` 的 GOROOT 绕过逻辑，`runCmd` 直接继承当前环境。磊哥机器 `~/.zshrc` 的 GOROOT 本就正确，问题来自 Pi 进程继承的旧环境变量，不属于产品要处理的场景。
4. 按方案第 4 节，前端标题使用项目名：`index.html` 的 `<title>`、登录页品牌标题/副标题（当前为"管理后台"/"Taurus Admin"）、首页顶栏品牌文字、`package.json` 的 `name`。`.vue` 文件仍不得走 Go 模板渲染，请用 `.env`/`import.meta.env`（如 `VITE_APP_TITLE`，由 `.tmpl` 生成的 env 文件提供）或其他不需要渲染 `.vue` 的方式。
5. `internal/cli`：输出目录已存在且非空时，在"输出目录"这一步就校验并提示（重新输入或报错退出均可），不要等到组件选择和确认之后才报错。`Generate` 内的校验保留。

验证：`go vet ./... && go test ./...` 通过；三种组合仍 `go build` 通过；全选项目前端 `pnpm build` 通过。不要启动服务或 Docker（运行验收由 Pi 执行）。完成后报告每项的改法与验证结果。
