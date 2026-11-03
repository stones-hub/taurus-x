# 压测工具（scripts/loadtest）

对**登录**和**后台列表接口**做有限时长、有限并发的压测，输出请求数、各类失败、
QPS 和 P50/P95/P99。独立 Go module、只用标准库，`go run` 直接跑，不需要安装任何工具，
也不进 `backend/go.mod`。

> 只对**你自己的测试环境**压测。压测会真实写库：每次登录写一条登录日志、更新最后登录信息；
> 每个列表请求写一条访问日志；每次登录在 Redis 里建一个会话。不要对生产环境跑。

## 一、怎么跑

```bash
cd scripts/loadtest
# 账号只从环境变量读，不接受命令行参数（命令行会出现在 ps 和 shell 历史里）。
# 下面的 read -s 从键盘读密码、不回显。
export LOADTEST_USERNAME=<测试账号>
read -rs LOADTEST_PASSWORD && export LOADTEST_PASSWORD

go run . -scenario login -duration 30s -concurrency 1
go run . -scenario list  -duration 30s -concurrency 4 -list-path /admin/user/list
unset LOADTEST_PASSWORD
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `-scenario` | 必填 | `login` 或 `list` |
| `-base-url` | `http://127.0.0.1:9080` | 只能是 `scheme://host:port`。默认**只允许回环地址**（`127.0.0.1` / `::1` / `localhost`） |
| `-allow-non-loopback` | 关 | 压测非本机地址时必须显式加上；只对你有权压测的测试环境用 |
| `-duration` | `10s` | 上限 `10m` |
| `-concurrency` | `4` | 并发 worker 数，上限 `64` |
| `-timeout` | `5s` | 单个请求超时（含读响应体），上限 `60s` |
| `-max-body` | `1048576` | 单个响应体上限（1KiB～8MiB），超出记为「响应异常」 |
| `-list-path` | `/admin/user/list` | 只允许内置的只读列表接口：用户 / 角色 / 权限 / 部门 / 登录日志 / 访问日志 / 操作日志 |
| `-page-size` | `15` | 列表请求的 `page_size`（1～200） |
| `-max-error-rate` | `0` | 允许的错误率（0～1），超过则退出码为 1；**429 计入错误** |
| `-logout` | `true` | 调 `/auth/logout` 吊销会话：登录成功后，以及登录判为失败但 jar 里已有 `jwt_token` 时（服务端可能已建会话）；登出失败计入「登出失败」且退出码为 1 |
| `-json` | 空 | 另存一份 JSON 报告（权限 0600）。先在同目录写临时文件再改名替换：写一半失败不留半份报告；目标是符号链接时替换的是链接本身，不会改写它指向的文件 |

退出码：`0` 跑完、错误率不超过阈值、会话全部登出；`1` 错误率超阈值、没有完成任何请求、**有会话没能登出**或被中断；
`2` 参数 / 环境变量不合法（没发请求）；`3` list 场景预登录失败或在预登录阶段被中断（没进入压测）。
用 `go run` 跑时，`go run` 会把程序的非 0 退出码统一变成 `1`（并打印 `exit status N`）；要拿到上面的原值，
先 `go build -o /tmp/loadtest .` 再直接执行编译出的程序。

自测：`cd scripts/loadtest && go test ./...`（用 httptest 模拟后端约定，不连真实服务）。

## 二、两个场景具体做什么

**login**：每次请求都用一个全新的 Cookie jar（等价一个新浏览器），按前端的方式
`POST /auth/login`：`Content-Type: application/json`、带 `Origin`、请求体 `{"username","password"}`。
成功的判定是 HTTP 200、`body.code == 200`，**并且**响应下发了 HttpOnly 的 `jwt_token` Cookie
和 `x-csrf-token` Cookie —— 只看 `code` 会把「接口回了成功但会话没建起来」算成成功。
成功后立即 `POST /auth/logout`（带 CSRF 头）吊销会话；登出不计入延迟样本，但占用 worker 时间，
所以 login 场景的 QPS 是「登录 + 登出」串行下的登录速率。

登出成功的判定比 `body.code` 更严：后端对「请求里根本没带令牌」的登出也回成功（只清 Cookie），
所以工具要求登出前本会话的 jar 里有会被发出去的 `jwt_token`、登出后它已被服务端清掉，否则记为登出失败。
登录被判为失败、但 jar 里已经有 `jwt_token` 时（比如响应缺 HttpOnly 或 CSRF Cookie，服务端可能照样建了会话），
同样会尝试登出；预登录失败时，失败的那一次也参与清理。缺 CSRF 时后端会拒绝吊销，这种情况如实记为登出失败、退出码为 1。
典型情况是 `JWT_COOKIE_SECURE=true` 时用 `http://` 压测**非回环**地址：Secure Cookie 不会发给 http 地址
（回环地址例外，按安全上下文处理），登录看起来成功，会话却没法吊销、留在 Redis 里——报告会提示，退出码为 1。

**list**：每个 worker 先各自登录一次，拿到**各自独立**的会话（N 个 worker = N 个浏览器 = N 次登录），
任何一个预登录失败就整体放弃、已建立的会话逐个登出、不进入压测（退出码 3）。
**预登录同样受登录限流约束**：默认每 IP 突发 10 次、之后每 6 秒回 1 次，所以默认配置下 `-concurrency` 超过 10
（或刚跑完 login 场景、令牌还没回满）时预登录必然被 429 拒绝。要用更高并发压列表，按第四节在测试环境临时放宽登录限流。之后每个 worker 用自己的 Cookie
反复 `POST <list-path>`，请求头带 `x-csrf-token`（值取自本会话的 CSRF Cookie，与前端一致），
请求体与前端列表页首次加载时一致：该页「不过滤」的条件（状态类字段为 `-1`，即 QUERY_ALL；发 `0` 会变成只查禁用的）+ `page_no=1` + `page_size`。**不发送 `Authorization` 头**：后端只认 Cookie。
压测期间**不刷新令牌**：会话默认 24 小时有效（`http.jwt.expire_hours`），远长于压测时长上限；如果会话中途失效
（比如被改密或禁用），请求会以业务失败（`body.code` 401）计入，工具不会自动重新登录掩盖它。
结束后逐个登出。

## 三、报告字段

| 字段 | 含义 |
|------|------|
| 成功 | HTTP 200 且 `body.code == 200`（login 另需 Cookie 齐全） |
| 业务失败 | HTTP 200 但 `body.code != 200`，按 code 细分。后端的 401/403/500 都是这种形态 |
| 限流 429 | HTTP 429，按 `Retry-After` 细分。**不算成功** |
| HTTP 错误 | 其它非 200 状态（如来源校验失败的 403、413、5xx、重定向——工具不跟随重定向） |
| 传输错误 | 连接失败、超时等，按 `timeout` / `connection_refused` / `connection_reset` / `other` 细分 |
| 响应异常 | 响应体超 `-max-body`、不是 `{code,...}` JSON，或登录成功却没下发 Cookie |
| 错误率 | 非成功 / 总数；没有任何请求时不计算（JSON 为 `null`），不会输出 NaN |
| QPS | 完成的请求数 / 实际测量时长。实际时长从压测开始到最后一个在途请求结束，**不含**准备阶段（list 的预登录），所以会略长于 `-duration` |
| 成功延迟 | 只统计成功请求（429 回得很快，混进来会把延迟拉低）。分位用 nearest-rank：升序排列，Pk 取第 ceil(k/100×N) 个；没有成功样本时不输出分位 |
| 登出失败 | 登出没成功的次数：这些会话会留在 Redis 里直到过期 |

截止时间和**第一次** Ctrl+C 只让 worker 不再发起新请求，在途请求照常等到响应或超时（最长 `-timeout`），
不会被误记为传输错误，然后照常登出、输出报告（标明「被中断」，退出码 1）。不想等在途请求时**再按一次** Ctrl+C，
进程立即结束（此时不输出报告，list 场景的会话也来不及登出，会留到过期）。预登录阶段按 Ctrl+C 会停止继续登录、
登出已建立的会话后退出。
报告里**没有**账号、密码、Cookie、令牌或响应体。

## 四、限流与结果解读（必读）

生成项目对这两类接口默认都按**来源 IP** 限流，而压测机通常只有一个 IP，所以在默认配置下
**量到的主要是限流器，不是服务容量**。报告里只要出现 429 就会提示这一点，并按错误计入。

| 接口 | 默认限流 | 配置位置 | 默认配置下的预期 |
|------|----------|----------|------------------|
| `/auth/login` | 每 IP 突发 10 次，之后每 6 秒回 1 个；全站突发 200 | `backend/config/autoload/http/http.yaml` 的 `http.rate_limit.sensitive.login.*`（无环境变量） | 桶满时开跑，D 秒内成功约 `10 + D/6` 次，其余是 429 |
| `/admin/**` 列表 | 每 IP 突发 300 次，之后每秒回 10 个 | 环境变量 `ADMIN_RATE_LIMIT_BURST` / `ADMIN_RATE_LIMIT_PER_SECOND` | D 秒内成功约 `300 + 10×D` 次 |

两种用法，按目的选：

1. **核对限流配置**：保持默认配置，用低并发（如 `-concurrency 1 -duration 30s`）跑，
   核对成功数是否与上表的预期一致、`Retry-After` 是否等于填充间隔。此时 429 是预期结果，
   退出码会是 1（或显式加 `-max-error-rate 1`）。
2. **测服务容量**：只在**独立测试环境**里**显式、临时**放宽限流，测完立即还原：
   - 后台接口：测试环境的 `.env.local` 里设 `ADMIN_RATE_LIMIT_BURST=0` 与 `ADMIN_RATE_LIMIT_PER_SECOND=0`（两项同时为 0 = 关闭）；
   - 登录：在测试部署的 `http.yaml` 里把 `sensitive.login` 的 `ip_capacity` / `global_capacity` 调大（`0` 表示「用内置默认值」，**不是**关闭）。

   不要改代码里的默认值，不要把放宽后的配置带进生产。

**验证码**：当前登录接口只校验用户名和密码，没有图形 / 滑块验证码，所以登录压测不需要也不会绕过任何验证码。
短信验证码只用于手机号绑定 / 解绑，不在压测范围内。以后如果给登录加了验证码，本工具**不支持**，
不要为了压测关闭验证码或跳过校验——那样量到的不是真实登录链路。

**密码散列**：登录用 bcrypt（cost 10，见 `backend/app/helper/crypto.go`）校验密码，单次约 50～100ms CPU，登录 QPS 的上限主要由它和 CPU 核数决定，
远低于列表接口是正常的。

## 五、基准记录

结果只有连同环境一起记录才有意义。下表**故意留空**：没有在你的机器上实跑过的数字不要填。

| 项 | 记录 |
|----|------|
| 日期 / 代码版本（commit） | |
| 机器（CPU 型号与核数、内存、OS） | |
| 后端运行方式（原生 / Docker）、`GOMAXPROCS` | |
| MySQL / Redis 版本与部署位置（同机 / 远程） | |
| 数据量（用户数、日志表行数） | |
| 限流配置（默认 / 放宽了哪些值） | |
| 命令（场景、并发、时长、超时、list-path、page-size） | |
| 结果：总数 / 成功 / 业务失败 / 429 / HTTP 错误 / 传输错误 / 响应异常 | |
| QPS（全部 / 成功） | |
| 成功延迟 P50 / P95 / P99 / max（样本数） | |
| 观察（CPU、日志队列丢弃告警、慢查询等） | |
