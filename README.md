# taurus-x

**用来生成 Go + Vue 管理后台项目的命令行脚手架。**

你运行一次 `taurus-x new <项目名>`，它就生成一份独立的项目代码，包含前后端代码、数据库初始化 SQL、Docker 文件，以及开发和部署文档。之后在生成的项目中开发业务，不是在这个脚手架仓库里开发业务。

> **第一次使用建议：只选默认的必选组件，先在本机跑通登录，再开始开发。**
>
> 当前还有未验证的场景和已知问题，详见 [已知限制](#12-已知限制与安全提醒)。

## 目录

- [1. 能生成什么](#1-能生成什么)
- [2. 脚手架是怎么实现的](#2-脚手架是怎么实现的)
- [3. 准备环境](#3-准备环境)
- [4. 安装脚手架](#4-安装脚手架)
- [5. 生成你的项目](#5-生成你的项目)
- [6. 在本机跑起来](#6-在本机跑起来)
- [7. 登录后怎么测试](#7-登录后怎么测试)
- [8. 部署](#8-部署)
- [9. 配置和命令速查](#9-配置和命令速查)
- [10. 项目生成后如何开发](#10-项目生成后如何开发)
- [11. 常见问题](#11-常见问题)
- [12. 已知限制与安全提醒](#12-已知限制与安全提醒)
- [13. 维护脚手架本身](#13-维护脚手架本身)

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
- 运行方式：前端编译为静态文件，由 Go 后端托管，一个 HTTP 端口提供页面和接口；开发时前端用 Vite 开发服务器。

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
| `milvus` | 否 | 向量数据库客户端；当前有已知阻塞，见[已知限制](#12-已知限制与安全提醒)，第一次使用不建议勾选 |

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

所以**新增 controller 或 service 时，只要按约定定义一个 `XxxSet` 变量，再执行 `make wire`，就会被自动接入**，不需要手工登记。生成文件的管理方式见第 10 节。

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

| 工具 | 用途 | 是否必须 |
| --- | --- | --- |
| Go 1.25 或兼容版本 | 安装 taurus-x；生成项目时执行依赖整理和 Wire；运行后端 | 必须 |
| Git | 从源码构建 taurus-x 时使用 | 可选 |
| Node.js | 运行前端；Vite 要求 20.19+ 或 22.12+，建议 22.12+ | 本机跑前端时必须 |
| pnpm | 前端包管理；本文用 `npm exec` 临时调用固定版本，不要求全局安装 | 同上 |
| MySQL 8.0 / Redis 7 | 后端运行时依赖；可用已有的独立服务，或用 Docker 临时起测试容器 | 本机跑后端时必须 |
| Docker | 只用来临时起测试用的 MySQL / Redis 容器 | 可选 |
| make | 使用 Makefile 快捷命令 | 可选，也可用本文的直接命令 |

先确认 Go 正常：

```bash
go version
```

首次生成需要下载 Go 依赖，首次运行前端需要下载前端依赖，请确保网络能访问相应的源。

## 4. 安装脚手架

### 4.1 用 go install 安装，推荐

安装最新发布版：

```bash
go install github.com/stones-hub/taurus-x@latest
```

也可以安装指定版本，所有版本见 GitHub 的 [Releases](https://github.com/stones-hub/taurus-x/releases) 页面：

```bash
go install github.com/stones-hub/taurus-x@v0.1.0
```

安装后命令通常位于 `$(go env GOPATH)/bin`，需要该目录在 `PATH` 中。如果提示找不到命令，先检查安装目录，不必重复安装。

确认安装的版本：

```bash
taurus-x version
```

查看帮助：

```bash
taurus-x help
```

### 4.2 升级脚手架

重新执行安装命令即可，新版本会覆盖旧的 `taurus-x`：

```bash
go install github.com/stones-hub/taurus-x@latest
```

注意：

- **升级只影响以后生成的新项目**。已经生成的项目是独立代码，不会随脚手架升级而改变；需要新版本的改进时，参考 Release 说明自行把改动迁移过去，不要对已有项目重新生成覆盖。
- 刚发布的新版本，`@latest` 可能因 Go 依赖代理（如 `goproxy.cn`）的缓存暂时还装到旧版本。此时直接写明版本号安装，或临时绕过代理：`GOPROXY=direct go install github.com/stones-hub/taurus-x@latest`。
- 装完用 `taurus-x version` 确认版本号。

### 4.3 从源码构建

需要修改模板，或要使用尚未发布的改动时，从源码构建：

```bash
git clone https://github.com/stones-hub/taurus-x.git && cd taurus-x
```

把 CLI 编译到临时目录，不往仓库根目录放二进制：

```bash
mkdir -p /tmp/taurusx-bin && go build -o /tmp/taurusx-bin/taurus-x .
```

之后用 `/tmp/taurusx-bin/taurus-x` 代替下文的 `taurus-x`。

> 模板通过 `embed` 打进二进制，改模板后必须重新构建。从源码构建时，`taurus-x version` 显示基于提交号的版本，例如 `v0.0.0-20261020065805-1e15e63db2fd`；有未提交的改动时末尾带 `+dirty`。

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

## 6. 在本机跑起来

生成后先在自己电脑上把项目跑起来、登录成功，再开始开发。后端直接运行在本机，前端用 Vite 开发服务器（支持热更新）；MySQL 和 Redis 自备。

> 这里是开发环境。部署到服务器（含 Docker 方式）请看生成项目的 `docs/部署文档.md`，见[第 8 节](#8-部署)。

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

这里只是本机测试容器的数据导入。数据库账号在本节为测试 root，仅用于本机开发。

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

- 本机后端连接测试容器时使用 `127.0.0.1` 和上面映射出来的端口。
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

### 6.7 用完后清理测试容器

先确认容器确实是自己本节创建的，再删除：

```bash
docker rm -f -v my-admin-test-mysql my-admin-test-redis
```

这会删除本节测试容器及其关联匿名卷中的数据。不要替换成其他项目容器名。

清除当前终端的测试密码变量：

```bash
unset MYSQL_ROOT_PASSWORD
```

## 7. 登录后怎么测试

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

如需反馈问题，请提供：操作步骤、期望结果、实际提示、浏览器控制台错误、脱敏后的相关服务日志。**不要提供密码、JWT、Cookie 或完整的 `.env.local`。**

## 8. 部署

部署不在本文范围内。生成项目自带 **`docs/部署文档.md`**，部署方式的选择、各方式的完整步骤和注意事项都在里面。

## 9. 配置和命令速查

### 配置文件分别管什么

| 文件 | 用途 |
| --- | --- |
| `backend/.env.example` | 本机后端配置示例，不含可用 JWT 密钥 |
| `backend/.env.local` | 本机运行后端时读取的实际配置，不提交 Git |
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

## 10. 项目生成后如何开发

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

- **与操作系统、CPU 架构无关**。内容只取决于选了哪些组件、写了哪些 controller/service，在哪台机器上生成都一样。
- **生成文件要提交到 Git**（Wire 官方推荐做法）。
- 生成项目时已自动执行一次；`make run`、`make build` 也会先自动重新生成。
- 新增或删除 controller/service 后，必须执行 `make wire`、确认 `go build ./...` 通过，并把生成文件一起提交。只拿仓库代码去编译的场合（例如构建 Docker 镜像）不会执行 Wire，漏提交会导致编译失败或漏掉新服务。
- 不要手工修改这些文件，下次 `make wire` 会覆盖。

在生成项目根目录重新生成并检查：

```bash
cd backend && make wire && go build ./...
```

## 11. 常见问题

### 生成时 `go mod tidy` 或 Wire 失败

先确认 Go 版本、网络、错误日志和输出目录。生成器会保留部分文件，不会自动清理失败现场。没有正确生成 Wire 代码的项目不能当作生成成功。

可以修复网络后在生成项目后端目录整理依赖并重跑 Wire；不要无脑清库或覆盖目录：

```bash
cd backend && go mod tidy && make wire
```

如需重新生成，使用新的空目录，或确认无需要保留的内容后自行清理旧测试目录。

### `JWT_SECRET` 相关报错

这是启动保护，不是要求关闭认证。用 `openssl rand -hex 32` 生成随机密钥，填入 `backend/.env.local` 的 `JWT_SECRET` 后重启后端。

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

先查明是谁在使用，不要停掉不属于本项目的进程。后端修改 `backend/.env.local` 的 `SERVER_PORT`，并同步改 `frontend/vite.config.ts` 的代理地址。

### 修改初始化 SQL 后数据库没有变化

`init.sql` 只在建库时导入一次，修改它不会改变已有的数据库。已有库请写增量 SQL 执行；只有自己的无价值测试数据才考虑删库重建。

### 构建出现 chunk 大小警告

前端可能提示 chunk 超过 500 kB。警告不等于构建失败；以退出码和产物为准，后续可按实际需求拆包优化。

## 12. 已知限制与安全提醒

**当前不是“所有场景均已验证的生产成品”。**

- 只选必选组件生成的项目已实测：生成、构建、初始化数据库、管理员登录、各基础页面、改密与角色禁用等流程通过。
- 勾选 `milvus` 后，生成项目的 Docker 编排依赖的 MinIO 镜像当前拉取失败，没有确认可用的替代版本；Consul、Milvus、OTEL 的真实连接未完成整体验收。
- 各部署方式已实测和未实测的环境，见生成项目 `docs/部署文档.md` 开头的验证状态说明。
- OTEL 默认关闭，需要你配置并启动 collector；TCP/gRPC 也不是选中后就自动带业务服务。
- 真实 MySQL 并发交错、完整安全故障矩阵等仍有未验证边界；普通功能通过不代表这些边界已通过。
- 密码散列目前仍是 `SHA-256(password + salt)`，不是 bcrypt/Argon2id；生成项目 README 还列有其他安全与功能待办，上生产前必须逐项评估。
- Redis 会话白名单缺失或故障会拒绝认证；改密等操作会吊销旧会话。这些现象不能简单理解为“登录偶尔不稳定”。
- 初始密码、JWT 密钥、数据库密码、Token、Cookie 不进 Git、不写入公开问题报告。
- 本机测试时服务尽量只监听 `127.0.0.1`；不要把临时数据库、缓存和测试后台暴露到公网。

进一步阅读：生成项目自带的 `README.md`（含「已知风险与待办」）和 `docs/` 下的文档。

## 13. 维护脚手架本身

本节面向修改 taurus-x 模板的人，普通使用者可跳过。

仓库结构与生成原理见第 2 节。

修改模板后必须重新构建 CLI，旧二进制不会自动带上新模板。

### 13.1 检查与测试

从脚手架仓库执行静态检查：

```bash
go vet ./...
```

执行测试：

```bash
go test -count=1 ./...
```

生成器测试覆盖必选、全选、典型组合，执行生成、依赖整理、Wire、后端 build/vet/test，并检查组件裁剪、模板残留、权限/路由一致性、安全回归及 Docker 跨平台文本约束。

测试不自动完成前端构建、真实数据库业务验收、Docker 镜像构建、浏览器验收或并发验证。改动模板后需要另外实际验证，不能拿改动前的验证结果代替。

遇到本机 Go 缓存权限问题，可为单次命令指定缓存目录，不改系统配置：

```bash
GOCACHE=/tmp/gocache-taurusx go test -count=1 ./...
```

### 13.2 发布新版本

使用者通过 `go install ...@latest` 或 `@vX.Y.Z` 安装，取的是仓库里的 Git tag。所以**发布新版本就是打一个新 tag**，再在 GitHub 上写一份 Release 说明。

**版本号规则**（`v主版本.次版本.修订号`）：

| 改动类型 | 例子 | 怎么升 |
| --- | --- | --- |
| 只修问题、改文档 | 修模板里的 bug、改错别字 | 修订号 +1：`v0.1.0` → `v0.1.1` |
| 新增功能 | 新增组件、新增底座页面 | 次版本 +1，修订号归零：`v0.1.1` → `v0.2.0` |
| 不兼容的改动 | 改命令行用法、改生成项目的目录结构 | 当前是 `v0.x` 阶段，同样升次版本；进入 `v1` 之后升主版本 |

**发布步骤**（在仓库根目录、`master` 分支执行）：

1. 确认要发布的改动都已提交并推送，工作区干净：

```bash
git status
```

2. 运行检查和测试，必须全部通过：

```bash
go vet ./... && go test -count=1 ./...
```

3. 打 tag 并推送，把 `v0.1.1` 换成新版本号：

```bash
git tag -a v0.1.1 -m "v0.1.1" && git push origin v0.1.1
```

4. 在 GitHub 创建 Release，写明这一版改了什么、使用者升级要注意什么。用 GitHub CLI 的话：

```bash
gh release create v0.1.1 --title "v0.1.1" --notes-file 发布说明.md
```

也可以在 GitHub 网页的 Releases 页面手动创建。Release 不需要附带编译好的程序，使用者用 `go install` 安装。

5. 实际安装一次确认：

```bash
GOPROXY=direct go install github.com/stones-hub/taurus-x@v0.1.1 && taurus-x version
```

注意：

- **tag 推送后不要删除或改指向**。Go 依赖代理会永久缓存每个版本的内容，改了也不会生效，还会让不同人装到的同一版本内容不一致。发错了就再发一个新版本。
- 版本号由 tag 决定，代码里不需要手工改版本号。

本仓库正式交付形态是 CLI 二进制；生成项目则由使用者按自己的 Git 和发布流程管理。
