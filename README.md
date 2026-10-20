# taurus-x

**用来生成 Go + Vue 管理后台项目的命令行脚手架。**

你运行一次 `taurus-x new <项目名>`，它就生成一份独立的项目代码，包含前后端、数据库初始化 SQL、Dockerfile 和部署文档。之后在生成的项目中开发业务，不是在这个脚手架仓库里开发业务。

> **第一次使用建议：只选默认的必选组件，先跑通登录。**
>
> 基础组合已经在本机 ARM Docker 环境验证过：生成 → 构建 → 启动 → 初始化数据库 → 管理员登录 → 读取身份 → 登出。
> 勾选 `milvus` 的完整 Docker 启动仍受 MinIO 镜像下载问题阻塞，不建议第一次测试时全选。
> 这不代表所有平台、可选组件或生产场景都已验证，详见 [第 13 节](#13-已知限制与安全提醒)。

## 目录

- [1. 能生成什么](#1-能生成什么)
- [2. 脚手架是怎么实现的](#2-脚手架是怎么实现的)
- [3. 准备环境](#3-准备环境)
- [4. 安装脚手架](#4-安装脚手架)
- [5. 生成你的项目](#5-生成你的项目)
- [6. 方式一：Docker 启动，推荐首次测试](#6-方式一docker-启动推荐首次测试)
- [7. 方式二：本机开发启动](#7-方式二本机开发启动)
- [8. 登录后怎么测试](#8-登录后怎么测试)
- [9. 部署到服务器](#9-部署到服务器)
- [10. 配置和命令速查](#10-配置和命令速查)
- [11. 项目生成后如何开发](#11-项目生成后如何开发)
- [12. 常见问题](#12-常见问题)
- [13. 已知限制与安全提醒](#13-已知限制与安全提醒)
- [14. 维护脚手架本身](#14-维护脚手架本身)

## 1. 能生成什么

### 自带功能

| 功能 | 说明 |
| --- | --- |
| 登录与身份认证 | 管理员登录、登出、Token 刷新、个人信息 |
| 用户管理 | 新建、编辑、状态管理、管理员重置用户密码 |
| 角色管理 | 用户分配角色、角色分配权限 |
| 权限管理 | 菜单、按钮、接口三级 RBAC |
| 部门管理 | 部门与部门成员 |
| 日志 | 登录、访问、操作、前端错误四类日志 |
| 个人中心 | 编辑个人资料、使用旧密码修改自己的密码 |
| 项目基础设施 | 配置、日志、MySQL、Redis、Wire 依赖注入、前端构建与托管 |

**不包含订单、客户、财务等业务模块，也不是业务代码自动生成器。** 文档中的业务示例不等于已实现的功能。

### 技术栈

- 后端：Go 1.25、Taurus Pro 组件、GORM、Google Wire。
- 前端：Vue 3、TypeScript、Vite、Element Plus、Pinia。
- 数据库与缓存：MySQL 8.0、Redis 7。
- 生产式运行：前端编译为静态文件，由 Go 后端托管，一个 HTTP 端口提供页面和接口。

### 组件怎么选

| 组件 | 是否必选 | 作用与选择建议 |
| --- | --- | --- |
| `common` | 是 | 日志、JWT、公共工具 |
| `config` | 是 | 配置加载、环境变量替换 |
| `http` | 是 | HTTP 服务和中间件 |
| `storage` | 是 | MySQL 和 Redis，后台底座必须使用 |
| `consul` | 否 | 服务注册发现；确有服务发现需求再选 |
| `opentelemetry` | 否 | 链路追踪、指标上报；需另备 collector |
| `tcp` | 否 | TCP 服务基础设施；需自己注册业务 handler |
| `grpc` | 否 | gRPC 服务基础设施；需自己注册业务 service |
| `milvus` | 否 | 向量数据库客户端；Docker 还依赖 etcd 和 MinIO，当前有镜像下载阻塞 |

必选组件即使取消，也会被生成器补回。未选的可选组件会从依赖、配置、组件注册和 Docker 编排中裁掉，不只是把开关关闭。

## 2. 脚手架是怎么实现的

一句话：**taurus-x 把一整套项目模板打包进命令行程序，运行时按你的选择把模板“填空”后写到新目录，再自动整理 Go 依赖、生成依赖注入代码。** 它不在运行时联网下载模板，生成出来的项目与 taurus-x 再无关联。

### 2.1 执行 `taurus-x new` 时发生了什么

```text
taurus-x new my-admin
  │
  ├─ 1. 交互收集输入（internal/cli）
  │     Go module 路径 → 输出目录 → 勾选组件 → 汇总确认
  │     项目名只能含字母、数字、_、-；输出目录必须不存在或为空
  │
  ├─ 2. 整理组件（internal/components）
  │     必选组件（common/config/http/storage）自动补齐，按注册表顺序排列
  │
  ├─ 3. 准备填空数据（internal/generator）
  │     项目名、module、数据库名（- 换成 _）、Go 版本、选中了哪些组件
  │     随机生成初始管理员密码和 salt，算出散列
  │
  ├─ 4. 渲染并写出全部文件
  │     遍历内置模板；.tmpl 文件填空，其余原样复制；未选组件的配置目录整个跳过
  │
  ├─ 5. 在 backend/ 执行 go mod tidy（下载并锁定依赖）
  │
  ├─ 6. 生成 Wire 依赖注入代码（见 2.4）
  │
  └─ 7. 打印下一步操作，以及只显示这一次的初始管理员密码
```

任何一步失败都会停止并报错，**已生成的文件保留在原地便于排查，不会自动删除**。

### 2.2 模板是怎么变成项目的

- **模板随程序一起打包**：`templates/project/` 整个目录通过 Go 的 `embed` 编译进 taurus-x（`templates/templates.go`），所以改了模板必须重新编译 taurus-x 才会生效。
- **只有 `.tmpl` 后缀的文件会填空**：用 Go 标准库 `text/template` 渲染，输出时去掉 `.tmpl` 后缀。其他文件（如 `.vue`、`.ts`、`.css`）原样复制，避免和 Vue 自己的 `{{ }}` 语法冲突。
- **填空时用到的数据**：

| 模板变量 | 含义 | 示例 |
| --- | --- | --- |
| `.ProjectName` | 项目名，用于程序名、容器名、页面标题 | `my-admin` |
| `.ProjectNameLower` | 小写项目名，用于 npm 包名 | `my-admin` |
| `.Module` | Go module 路径，用于所有 import | `github.com/stones-hub/my-admin` |
| `.DBName` | 默认数据库名和账号名 | `my_admin` |
| `.GoVersion` | 生成项目 `go.mod` 声明的 Go 版本 | `1.25.1` |
| `.Has.<组件>` | 是否选了某个组件，用于条件裁剪 | `{{ if .Has.milvus }}` |
| 初始管理员散列与 salt | 只写进 `init.sql` | - |

- **引用了不存在的变量会直接报错**（`missingkey=error`），不会悄悄生成一个空值。
- 少数文件为了不影响 taurus-x 自己的仓库而改了名，输出时还原，例如模板里的 `gitignore` 输出为 `.gitignore`。

### 2.3 组件是怎么裁剪的

组件清单和固定版本登记在 `internal/components/components.go`。没选的组件会从下面这些地方一起去掉，而不是只把开关关掉：

| 位置 | 裁剪内容 |
| --- | --- |
| `backend/go.mod` | 不引入该组件的依赖包 |
| `backend/internal/taurus/` | 不注册该组件的初始化代码 |
| `backend/config/autoload/<组件>/` | 整个配置目录不生成 |
| `backend/app/bootstrap.go` | 不启动对应的 TCP / gRPC 服务 |
| `docker-compose.yml`、`Dockerfile` | 不编排对应的容器、不暴露对应端口 |
| `README.md`、`docs/部署文档.md` | 不出现该组件的说明 |

### 2.4 依赖注入代码是怎么生成的

生成项目用 Google Wire 把各个模块连起来。代码分两层生成，都通过 `go run` 调用，**不需要全局安装 Wire**，版本由生成项目的 `go.mod` 锁定：

1. **框架组件层**（`backend/internal/taurus/`）：配置、日志、MySQL、Redis、HTTP 等组件如何初始化，生成 `internal/taurus/wire_gen.go`。
2. **业务层**（`backend/app/`）：扫描器（`backend/internal/wire/scanner/`）读取 `app/` 下所有 Go 文件，找出名字以 `Set`、`ProviderSet` 或 `WireSet` 结尾、用 `wire.NewSet(...)` 定义的变量，据此生成 `app/wire.go`，再由 Wire 生成 `app/wire_gen.go`。

所以**新增 controller 或 service 时，只要按约定定义一个 `XxxSet` 变量，再执行 `make wire`，就会被自动接入**，不需要手工登记。生成文件的管理方式见第 11 节。

### 2.5 生成出来的项目运行时是什么样

**启动顺序**（`backend/bin/main.go` → `backend/app/bootstrap.go`）：

1. 读取启动参数 `-config ./config -env .env.local`。
2. 检查 `-env` 指定的文件存在且可读，读不到直接退出。
3. 加载 `config/` 下的配置，用环境变量替换其中的占位符。
4. 初始化框架组件：日志、MySQL、Redis、HTTP，以及选中的可选组件。
5. 安全自检：`JWT_SECRET` 未配置、过短或是弱值时拒绝启动。
6. 用 Wire 生成的代码组装业务层：数据访问（repo）→ 业务逻辑（service）→ 接口（controller）。
7. 注入权限检查器和登录状态校验，注册全部路由。
8. 启动 HTTP 服务（选了 TCP / gRPC 且已开启时一并启动），收到停止信号后优雅关闭。

**一次请求怎么走**：

```text
浏览器 ──> Go 服务（默认 9080）
            ├─ /health                健康检查
            ├─ /auth/login、/logout   登录登出（带请求体上限、登录限流）
            ├─ /admin/... 等接口      依次经过：JWT 校验 → 会话校验 → 访问日志
            │                         → 接口权限校验 → 操作日志 → CSRF 校验 → 业务代码
            └─ 其它路径               返回前端页面（../frontend/dist，支持前端路由刷新）
```

- **登录状态**：JWT 只是凭证，每次请求还要到 Redis 里核对会话是否仍有效；改密码等操作会让旧会话立即失效。Redis 不可用时一律拒绝，不会放行。
- **权限**：权限分菜单、按钮、接口三类，存在 MySQL 的 `admin_permissions` 表。前端登录后从后端获取菜单和按钮权限来显示页面；**真正的拦截在后端**，接口路径必须与权限表里登记的路径完全一致。
- **前端**：开发时 Vite 把 `/api` 代理到后端；正式运行时前端编译成静态文件，由 Go 直接返回，一个端口同时提供页面和接口，运行时不需要 Node。

### 2.6 脚手架仓库结构

```text
taurus-x/
├── main.go                      入口，转交给 internal/cli
├── internal/cli/                命令行参数与交互提问
├── internal/components/         组件清单：名称、依赖包、固定版本、是否必选
├── internal/generator/          填空数据、渲染与写文件、tidy 与 Wire、生成器测试
│   └── password.go              初始管理员随机密码与散列
└── templates/
    ├── templates.go             把 project/ 打包进程序
    └── project/                 生成项目的完整模板（backend/、frontend/、scripts/、docs/ 等）
```

## 3. 准备环境

两种运行方式任选一种，**不要同时启动，避免端口和数据库冲突**。

| 工具 | Docker 方式 | 本机开发方式 |
| --- | --- | --- |
| Go 1.25 或兼容版本 | 必须，CLI 生成后会执行 Go 依赖整理与 Wire | 必须 |
| Git | 获取源码时使用 | 获取源码时使用 |
| Docker 与 Compose 插件 | 必须；macOS 可使用 Docker Desktop | 只在用容器提供 MySQL/Redis 时需要 |
| Node.js | 本机不要求，Docker 构建阶段自带 | 建议 Node 22.12+；Vite 要求 Node 20.19+ 或 22.12+ |
| pnpm | 本机不要求，Docker 构建阶段自带 | 本文使用 `npm exec` 临时调用固定版本，不要求全局安装 |
| MySQL 8.0 / Redis 7 | Compose 自动启动 | 自备独立服务，或使用测试容器 |
| make | 不要求 | 使用 Makefile 时需要，也可用本文直接命令 |

先确认 Go 正常：

```bash
go version
```

如果准备走 Docker 方式，启动 Docker 后检查：

```bash
docker version
```

```bash
docker compose version
```

首次生成和构建需要下载 Go 包、前端依赖和容器镜像，请确保网络可访问相应源。构建耗时会受网络与缓存影响。

## 4. 安装脚手架

### 3.1 用 go install 安装，推荐

```bash
go install github.com/stones-hub/taurus-x@latest
```

安装后命令通常位于 `$(go env GOPATH)/bin`，需要该目录在 `PATH` 中。如果提示找不到命令，先检查安装目录，不必重复安装。

查看帮助：

```bash
taurus-x help
```

### 3.2 从源码构建

需要修改模板，或要使用尚未发布的改动时，从源码构建：

```bash
git clone https://github.com/stones-hub/taurus-x.git && cd taurus-x
```

把 CLI 编译到临时目录，不往仓库根目录放二进制：

```bash
mkdir -p /tmp/taurusx-bin && go build -o /tmp/taurusx-bin/taurus-x .
```

之后用 `/tmp/taurusx-bin/taurus-x` 代替下文的 `taurus-x`。

> 模板通过 `embed` 打进二进制，改模板后必须重新构建；CLI 版本号不能区分本机未提交改动。

## 5. 生成你的项目

以下统一使用项目名 **`my-admin`**，数据库名为 **`my_admin`**。

> 换项目名时，请同步替换后续命令中的目录、容器名和数据库名。项目名中的 `-` 会在默认数据库名中转换为 `_`。

在你存放代码的目录执行。如果是从源码构建的，**不要在 taurus-x 仓库内部执行**，避免把新项目嵌套进脚手架仓库：

```bash
taurus-x new my-admin
```

按提示完成以下步骤：

| 提示 | 第一次测试怎么填 |
| --- | --- |
| Go module 路径 | 可接受默认 `github.com/stones-hub/my-admin`；正式项目换成自己的仓库路径 |
| 输出目录 | 接受默认 `./my-admin` |
| 组件选择 | 只保留四个必选组件；上下移动，空格勾选，回车确认 |
| 汇总确认 | 核对目录、module、组件后确认生成 |

项目名只能使用字母、数字、下划线、短横线。输出目录必须不存在或为空，生成器不会覆盖已有项目。

生成过程会自动：

1. 渲染前后端与配置模板。
2. 生成数据库初始化 SQL。
3. 在后端执行 `go mod tidy`。
4. 扫描 provider 并执行 Wire 代码生成，不需要全局安装 Wire。
5. 打印下一步操作和初始管理员账号密码。

**务必保存终端打印的初始密码。**

- 初始用户名：`admin`。
- 初始密码：每次随机生成，只在这次终端输出中打印一次。
- 密码不会写入生成项目的 README 或明文配置中；SQL 里只保存散列。
- 不要把包含初始密码的截图、终端记录公开或提交到 Git。
- 如果生成过程失败，目录可能已生成部分内容，先看错误；不要直接往非空目录反复生成。

进入生成项目：

```bash
cd my-admin
```

**后文命令如无特别说明，都在生成项目根目录 `my-admin` 下执行。**

主要目录如下：

```text
my-admin/
├── backend/                 Go 后端、配置、Wire、后端 Makefile
├── frontend/                Vue 前端
├── scripts/sql/init.sql     初始表、角色、权限、管理员凭证散列
├── docs/                    生成项目的开发与部署说明
├── Dockerfile               前后端多阶段构建
├── docker-compose.yml       所选组件的容器编排
├── Makefile                 前后端快捷命令
└── README.md                生成项目自己的说明
```

## 6. 方式一：Docker 启动，推荐首次测试

这是在自己电脑上用 Docker 试跑，不是服务器部署。**本节命令都在生成项目根目录执行，不是在 taurus-x 仓库执行。**

适用场景：想先验证“生成后能不能登录”，暂时不需要前端热更新。建议只用必选组件。

### 5.1 创建 Compose 环境配置

用编辑器在生成项目根目录创建 **`.env`**，填入以下配置。它用于 Compose 变量替换，**不是** `backend/.env.local`。

下面的密码只是示范，请自己替换；`MYSQL_PASSWORD` 和 `DB_DSN` 中的数据库密码必须相同：

```dotenv
MYSQL_ROOT_PASSWORD=ReplaceWithYourLocalRootPassword
MYSQL_PASSWORD=ReplaceWithYourLocalDbPassword
DB_DSN=my_admin:ReplaceWithYourLocalDbPassword@tcp(mysql:3306)/my_admin?charset=utf8mb4&parseTime=True&loc=Local
JWT_SECRET=替换为下一条命令生成的随机值
AUTHORIZATION=替换为另一个随机值
```

分别执行以下命令生成随机值，再用编辑器粘贴到对应字段中：

```bash
openssl rand -hex 32
```

```bash
openssl rand -hex 24
```

注意：

- `JWT_SECRET` 必须配置；留空、过短或弱值会导致启动失败。
- 示例假设项目名为 `my-admin`，默认数据库与普通数据库用户名均为 `my_admin`。
- `DB_DSN` 中使用 Compose 内部主机名 `mysql`，**不是** `127.0.0.1`。
- 第一次测试建议使用字母数字密码，避免 `$`、`#`、引号等引发 Compose 插值或解析歧义。
- `.env` 属于本机秘密文件，不要提交到 Git，也不要发到聊天或问题报告中。

收紧文件权限：

```bash
chmod 600 .env
```

### 5.2 把测试入口改为本机独立端口

模板默认发布 `9080:9080`，并监听所有网卡。本机测试建议改成只监听 `127.0.0.1`；如果 9080 已被其他服务占用，也需要换端口。

用编辑器打开生成项目的 `docker-compose.yml`，在 **`app` 服务的 `ports`** 中，把：

```yaml
- "9080:9080"
```

改成：

```yaml
- "127.0.0.1:19084:9080"
```

这样浏览器访问本机 `19084`，容器内仍使用 `9080`；只有本机能够访问这个测试入口。

可先检查端口是否空闲（没有输出通常表示没有监听进程）：

```bash
lsof -nP -iTCP:19084 -sTCP:LISTEN
```

只选必选组件时，MySQL、Redis 默认仅在 Compose 网络内访问，不需要映射宿主机端口。

### 5.3 准备日志目录

应用容器以非 root 的 `1000:1000` 运行，宿主机挂载目录需允许它写入。

```bash
mkdir -p docker-data/logs
```

Linux 或需要调整属主的环境，按生成部署文档设置：

```bash
sudo chown -R 1000:1000 docker-data
```

该命令只针对当前生成项目的 `docker-data`，不要扩大到整个仓库或其他目录。macOS Docker 文件共享的权限表现可能不同；遇到问题先查看应用日志，不建议用 `chmod 777` 粗暴处理。

### 5.4 检查配置并启动

先验证 Compose 配置完整，避免把密码等解析结果打印出来：

```bash
docker compose config --quiet
```

构建镜像并启动：

```bash
docker compose up -d --build
```

首次启动要等待数据库初始化。查看服务状态：

```bash
docker compose ps
```

查看最近日志：

```bash
docker compose logs --tail=100 app mysql redis
```

分享日志前请自行脱敏，不要发完整环境配置、Token 或初始密码。

### 5.5 打开页面并登录

检查应用健康：

```bash
curl -fsS http://127.0.0.1:19084/health
```

打开浏览器：

**http://127.0.0.1:19084/login**

用 `admin` 和生成时打印的随机密码登录。页面能打开只代表静态页面可用，**成功登录并进入首页才算认证流程跑通**。

第一次创建数据库卷时，MySQL 会自动执行 `scripts/sql/init.sql`。以后重启不会再次导入；修改 SQL 文件也不会自动修改已有数据库。

### 5.6 停止、恢复、清空

仅停止容器，保留数据：

```bash
docker compose stop
```

恢复已有服务：

```bash
docker compose up -d
```

删除本项目容器与网络，保留数据库卷：

```bash
docker compose down
```

**彻底清空本项目数据库、Redis 数据后重建：以下命令会删除数据，仅用于你自己的测试项目。**

```bash
docker compose down --volumes
```

再执行启动命令会重新初始化数据库，恢复到生成 SQL 中的初始状态。不要对已有真实业务数据的项目执行清卷操作。

## 7. 方式二：本机开发启动

这是在自己电脑上开发，不是服务器部署。适用场景：需要修改前后端、前端热更新。后端直接运行在本机；MySQL 和 Redis 自备。

**不要使用、重置或清空其他项目正在运行的数据库。** 以下示例用独立容器与端口，不影响你已有的数据库。

### 6.1 准备独立 MySQL 和 Redis

本机已有可用的独立服务可跳过本步，按真实连接信息填写配置。

示例创建测试 MySQL；`-e MYSQL_ROOT_PASSWORD` 从当前终端环境读取，不把密码直接写进命令。先在终端设置一个仅测试使用的值（注意命令历史；也可通过终端的安全输入方式设置）：

```bash
export MYSQL_ROOT_PASSWORD=ReplaceWithYourLocalRootPassword
```

```bash
docker run -d --name my-admin-test-mysql -p 127.0.0.1:23309:3306 -e MYSQL_ROOT_PASSWORD -e MYSQL_DATABASE=my_admin mysql:8.0
```

启动独立 Redis：

```bash
docker run -d --name my-admin-test-redis -p 127.0.0.1:26382:6379 redis:7-alpine
```

确认 MySQL 已就绪；命令会提示输入数据库 root 密码：

```bash
docker exec -it my-admin-test-mysql mysqladmin -uroot -p ping
```

### 6.2 首次导入 SQL

从生成项目根目录，把初始化 SQL 放到本次 MySQL 测试容器：

```bash
docker cp scripts/sql/init.sql my-admin-test-mysql:/tmp/my-admin-init.sql
```

导入，按提示输入刚才设置的数据库 root 密码：

```bash
docker exec -it my-admin-test-mysql mysql -uroot -p my_admin -e 'source /tmp/my-admin-init.sql'
```

清理容器内临时 SQL：

```bash
docker exec my-admin-test-mysql rm -f /tmp/my-admin-init.sql
```

这里只是本机测试容器的数据导入，不是生产部署方式。数据库账号在本节为测试 root；真实部署应使用最小权限业务账号。

**不要重复向已有数据的库盲目导入初始化 SQL。** 表使用建表保护不等于所有初始化数据都可以重复执行。

### 6.3 配置本机后端

从生成项目根目录复制配置示例：

```bash
cp backend/.env.example backend/.env.local
```

用编辑器修改 `backend/.env.local` 中以下字段；其他字段保留示例结构即可：

```dotenv
SERVER_ADDRESS=127.0.0.1
SERVER_PORT=19084
DB_NAME=my_admin
DB_DSN=root:ReplaceWithYourLocalRootPassword@tcp(127.0.0.1:23309)/my_admin?charset=utf8mb4&parseTime=True&loc=Local
REDIS_HOST=127.0.0.1
REDIS_PORT=26382
REDIS_PASSWORD=
JWT_SECRET=替换为openssl生成的随机值
AUTHORIZATION=替换为另一个随机值
```

生成 JWT 密钥：

```bash
openssl rand -hex 32
```

设置配置文件权限：

```bash
chmod 600 backend/.env.local
```

配置注意：

- 本机后端连接容器时使用 `127.0.0.1` 和映射端口，不使用 Compose 内部主机名 `mysql`。
- 配置值会先做文本替换再解析，**不要给值额外加引号，不要包含 `}` 或换行**。
- 空 Redis 密码写 `REDIS_PASSWORD=`，不要写成 `REDIS_PASSWORD=""`。
- 本节后端改用了 `19084`，前端代理也必须对应修改。

### 6.4 修改前端代理

用编辑器打开 `frontend/vite.config.ts`，把 `/api` 代理的：

```typescript
target: 'http://127.0.0.1:9080',
```

改为：

```typescript
target: 'http://127.0.0.1:19084',
```

否则页面能打开，但登录请求会发往错误的后端。

### 6.5 安装前端依赖

进入前端目录：

使用固定版本 pnpm 安装，不需要全局安装：

```bash
cd frontend && npm exec --yes pnpm@10.34.7 -- install
```

安装完成后回到项目根目录：

```bash
cd ..
```

### 6.6 分两个终端启动

**终端一：启动后端。**

```bash
cd backend && make run
```

`make run` 会先执行 Wire，再读取 `.env.local` 启动后端。

**终端二：启动前端。**

```bash
cd frontend && npm exec --yes pnpm@10.34.7 -- dev --host 127.0.0.1
```

两个终端都要先进入 `my-admin` 项目根目录再执行。

打开前端终端实际打印的地址，通常是：

**http://127.0.0.1:5173/login**

如果 `5173` 已占用，Vite 可能选择其他端口，以终端显示为准。后端健康地址为 `http://127.0.0.1:19084/health`。

测试结束，在两个终端分别按 `Ctrl+C` 停止服务。

### 6.7 验证生产式前端托管

先停止开发服务器，构建前端：

```bash
cd frontend && npm exec --yes pnpm@10.34.7 -- build && cd ..
```

构建后端：

```bash
cd backend && make build
```

从 **backend 目录**运行编译产物：

接上一步，仍在 `backend` 目录中执行：

```bash
./build/my-admin -config ./config -env .env.local
```

访问 **http://127.0.0.1:19084/login**。此时不依赖 Vite，前端文件来自 `frontend/dist`；不要只复制后端二进制而遗漏前端产物与配置。

### 6.8 清理本节测试容器

先确认容器确实是自己本节创建的，再删除：

```bash
docker rm -f -v my-admin-test-mysql my-admin-test-redis
```

这会删除本节测试容器及其关联匿名卷中的数据。不要替换成其他项目容器名。

清除当前终端的测试密码变量：

```bash
unset MYSQL_ROOT_PASSWORD
```

## 8. 登录后怎么测试

建议按这个顺序，先验证最小闭环，再增加组件或业务。

| 步骤 | 操作 | 预期 |
| --- | --- | --- |
| 1 | 使用初始管理员登录 | 进入首页，无接口错误 |
| 2 | 打开用户、角色、权限、部门页面 | 页面能加载，列表正常 |
| 3 | 打开登录、访问、操作、前端错误日志 | 列表正常；没有数据时可以为空 |
| 4 | 点击顶栏个人中心按钮 | 打开个人资料与改密表单 |
| 5 | 用旧密码修改自己的密码 | 修改成功；旧会话后续请求失效，需要新密码重新登录 |
| 6 | 新建测试角色并分配菜单、按钮、接口权限 | 保存成功，三类权限用途不同 |
| 7 | 新建测试用户并分配该角色 | 使用不同的合法手机号，填写符合校验要求的密码 |
| 8 | 用无痕窗口登录测试用户 | 只看到有权限的功能，不与管理员会话混用 |
| 9 | 管理员禁用测试角色 | 普通用户后续请求不再获得该角色权限；菜单重新加载后相应更新 |
| 10 | 管理员编辑该测试用户，只改昵称 | 已持有的禁用角色能回显并保留；不能新增未持有的禁用角色 |
| 11 | 重新启用测试角色 | 关联仍保留时，其有效权限恢复 |

补充说明：

- 接口有些业务失败仍使用 HTTP 200，排查时还要看响应 JSON 的 `code` 和错误提示。
- 新建用户的 `username`、`password`、`mobile` 都需要按实际表单填写；不要漏手机号。
- 普通用户能改自己的密码，不意味着能管理别人的账号。
- 普通管理员即使具有部分用户管理接口权限，也不能管理超级管理员账号。
- 角色被禁用不等于删除关联；已有关系可以保留，启用后恢复，但仍受权限行状态约束。
- 页面没有自动刷新时，可退出重新登录或刷新页面后再检查菜单，不要只看旧页面上已经渲染的按钮。

如需反馈问题，请提供：操作步骤、期望结果、实际提示、浏览器控制台错误、脱敏后的相关服务日志。**不要提供密码、JWT、Cookie 或完整 `.env`。**

## 9. 部署到服务器

本文只讲脚手架怎么用，**部署步骤不在这里**。生成项目里自带 `docs/部署文档.md`，按它操作即可：它说明了 Docker、Ubuntu 服务器、macOS 三种部署方式分别适合什么场景、完整步骤、CPU 架构处理、可选的 nginx，以及哪些环境尚未实测。

第 6、7 节是在自己电脑上试跑和开发，不是部署方式。

## 10. 配置和命令速查

### 配置文件分别管什么

| 文件 | 用途 |
| --- | --- |
| 项目根目录 `.env` | Docker Compose 插值，例如数据库密码、JWT 密钥 |
| `backend/.env.example` | 本机后端配置示例，不含可用 JWT 密钥 |
| `backend/.env.local` | 本机直接运行后端时读取的实际配置，不提交 Git |
| `backend/config/` | 应用配置模板，环境变量代入后加载 |
| `frontend/vite.config.ts` | 开发服务器端口及后端代理地址 |
| `scripts/sql/init.sql` | 首次建表、初始化角色权限和管理员散列 |

### 生成项目根目录的 Makefile

| 命令 | 用途 |
| --- | --- |
| `make backend-wire` | 重新生成后端依赖注入代码 |
| `make backend-run` | Wire 后启动后端 |
| `make backend-build` | Wire 后编译后端二进制 |
| `make frontend-install` | 安装前端依赖，要求命令行能找到 pnpm |
| `make frontend-dev` | 启动前端开发服务器，要求 pnpm |
| `make frontend-build` | 前端类型检查与构建，要求 pnpm |
| `make dev` | 同时启动前后端，要求 pnpm；本文推荐两终端，便于看日志和退出 |
| `make build` | 编译后端与前端，要求 pnpm |

Makefile 的提示文字使用默认端口；你改过配置时，应以实际监听地址为准。没有全局 pnpm 时，直接使用本文的 `npm exec --yes pnpm@10.34.7 -- ...`。

## 11. 项目生成后如何开发

1. **把生成项目当作独立项目管理**：进入它的目录，自行初始化 Git、配置自己的远程仓库；生成器不会自动替你提交或发布。
2. **先跑通原始底座，再加业务**，避免把环境问题和新业务问题混在一起。
3. 后端业务主要在 `backend/app/` 中扩展；新增依赖注入项后重新执行 Wire。
4. 新业务表使用明确 SQL/迁移管理；本底座不开启 GORM AutoMigrate。
5. 前端在 `frontend/src/` 中扩展页面、API 与路由；界面按钮隐藏不能替代后端接口权限校验。
6. 增加菜单、按钮、接口权限时，同时维护权限数据和调用链。具体分层以生成项目 `docs/` 的开发文档为准。
7. **生成后不会自动同步模板更新**。以后升级 taurus-x 不会直接修改已生成项目；已有项目需要审查后自行迁移改动，不要向非空项目重新生成覆盖。

关于现有用户更新接口：`/admin/user/update` 不是通用 PATCH。只有 `role_ids` 支持选择性更新：不传或 `null` 表示不改角色，`[]` 表示清空角色；其他资料仍按完整表单提交。仅调整角色可用已有 `/admin/role/assign`。

### Wire 生成文件怎么管

`backend/app/wire.go`、`backend/app/wire_gen.go`、`backend/internal/taurus/wire_gen.go` 是依赖注入的生成代码：

- **与操作系统、CPU 架构无关**。内容只取决于选了哪些组件、写了哪些 controller/service，不含任何平台相关代码；在 macOS 生成、提交的文件，可以直接在 Ubuntu、ARM 或 AMD 上编译。区分平台的是编译阶段的 `GOOS/GOARCH`，Dockerfile 已按目标平台编译。
- **生成文件要提交到 Git**（Wire 官方推荐做法）。部署机和 Docker 构建不需要安装 Wire，也不需要为此联网。
- 生成项目时已自动执行一次；`make run`、`make build` 也会先自动重新生成。
- **Docker 构建不会执行 Wire**，只使用仓库里已有的生成文件。新增或删除 controller/service 后，必须先执行 `make wire`、确认 `go build ./...` 通过，并把生成文件一起提交；否则镜像可能编译失败或漏掉新服务。
- 不要手工修改这些文件，下次 `make wire` 会覆盖。

在生成项目根目录重新生成并检查：

```bash
cd backend && make wire && go build ./...
```

## 12. 常见问题

### 生成时 `go mod tidy` 或 Wire 失败

先确认 Go 版本、网络、错误日志和输出目录。生成器会保留部分文件，不会自动清理失败现场。没有正确生成 Wire 代码的项目不能当作生成成功。

可以修复网络后在生成项目后端目录整理依赖并重跑 Wire；不要无脑清库或覆盖目录：

```bash
cd backend && go mod tidy && make wire
```

如需重新生成，使用新的空目录，或确认无需要保留的内容后自行清理旧测试目录。

### `JWT_SECRET` 相关报错

这是启动保护，不是要求关闭认证。生成随机密钥填入正确配置文件：Docker 用根目录 `.env`，本机运行用 `backend/.env.local`，再重新创建容器或重启本机后端。

### 页面打开了，但登录失败

按顺序检查：

1. 使用的是本次生成时打印的密码，而不是另一个项目的随机密码。
2. MySQL 是否导入了当前项目的 `init.sql`。
3. 旧数据库卷是否保留了另一次生成的账号散列。
4. 本机开发时，Vite 代理是否指向实际后端端口。
5. Redis 是否可连接；会话认证依赖 Redis，故障时会拒绝请求。
6. 浏览器请求响应的业务 `code`，不要只看 HTTP 状态。

### 初始密码忘记了

散列不能反推出明文。**还没有业务数据的测试项目，最简单的是重新生成新项目并使用新的独立数据库。**

已有数据时不要删库，需要走经过审查的管理员密码恢复流程；不要随意改库绕过账号吊销逻辑，也不要把密码或散列公开给别人排查。

### 端口被占用

先查明是谁在使用，不要停掉不属于本项目的进程。Docker 修改宿主机映射的左侧端口；本机后端修改 `SERVER_PORT`，并同步改 Vite 代理。

### 应用容器反复重启

先看 `docker compose logs --tail=100 app`。常见原因是 JWT 未配置、数据库连接参数不一致、日志挂载目录不可写。基础服务的 `running` 状态不等于应用业务已就绪，应检查 `/health` 和实际登录。

### 修改初始化 SQL 后数据库没有变化

MySQL 仅在空数据目录首次初始化时执行脚本。已有库要用正式迁移；仅自己的无价值测试数据才考虑清卷重建。

### 勾选 Milvus 后下载镜像失败

当前模板固定的 `minio/minio:RELEASE.2024-08-17T01-24-54Z` 在本次环境不能匿名拉取，官方 Quay 同 tag 也未取得可用 manifest。**不是已确认“换前缀就能修好”。**

首次测试请新生成一个只选必选组件的项目，不建议随手改成 `latest`、第三方镜像或认为仅关闭应用配置就能让 Compose 自动跳过依赖服务。

### 构建出现 chunk 大小警告

前端可能提示 chunk 超过 500 kB。警告不等于构建失败；以退出码和产物为准，后续可按实际需求拆包优化。

## 13. 已知限制与安全提醒

**当前不是“所有场景均已验证的生产成品”。**

- 当前代码的基础 Docker 组合已在本机原生 arm64 验证；不等于 amd64 或所有跨平台产物均已实际运行。
- Dockerfile 使用 BuildKit 目标平台编译；构建成功不等于目标机器运行成功，跨架构交付需另行验收。
- 全选 Compose 仍被 MinIO 下载阻塞；Consul、Milvus、OTEL 的真实连接未完成整体验收。
- OTEL 默认关闭，需要你配置并启动 collector；TCP/gRPC 也不是选中后就自动带业务服务。
- 真实 MySQL 并发交错、完整安全故障矩阵等仍有未验证边界；普通功能通过不代表这些边界已通过。
- 密码散列目前仍是 `SHA-256(password + salt)`，不是 bcrypt/Argon2id；生成项目 README 还列有其他安全与功能待办，上生产前必须逐项评估。
- Redis 会话白名单缺失或故障会拒绝认证；改密等操作会吊销旧会话。这些现象不能简单理解为“登录偶尔不稳定”。
- `.env.local` 缺失或读不到时，程序会在启动自检阶段直接退出（日志写明文件路径和处理方式），进程管理工具显示 `FATAL` / `BACKOFF`；不会再出现「显示运行中但端口不通」。排查方法见部署文档 FAQ。
- 初始密码、JWT 密钥、数据库密码、Token、Cookie 不进 Git、不写入公开问题报告。
- 只在本机测试时尽量绑定 `127.0.0.1`；不要把临时数据库、缓存和测试后台暴露到公网。

进一步阅读：生成项目自带的 `README.md`（含「已知风险与待办」）和 `docs/` 下的开发、部署文档。

## 14. 维护脚手架本身

本节面向修改 taurus-x 模板的人，普通使用者可跳过。

仓库结构与生成原理见第 2 节。

修改模板后必须重新构建 CLI，旧二进制不会自动带上新模板。

### 检查与测试

从脚手架仓库执行静态检查：

```bash
go vet ./...
```

执行测试：

```bash
go test -count=1 ./...
```

生成器测试覆盖必选、全选、典型组合，执行生成、依赖整理、Wire、后端 build/vet/test，并检查组件裁剪、模板残留、权限/路由一致性、安全回归及 Docker 跨平台文本约束。

测试不自动完成前端构建、真实数据库业务验收、Docker 镜像构建、浏览器验收或并发验证。这些应单独对当前新候选执行，不用旧候选的结果冒充。

遇到本机 Go 缓存权限问题，可为单次命令指定缓存目录，不改系统配置：

```bash
GOCACHE=/tmp/gocache-taurusx go test -count=1 ./...
```

本仓库正式交付形态是 CLI 二进制；生成项目则由使用者按自己的 Git、发布和部署流程管理。
