# Handover

## 当前任务与方案

- 当前里程碑/任务：T1 脚手架跑通 —— **本机候选验收通过**，用户已授权提交，本次提交保存 T1 基线；下一步 T2 安全修复（关键任务，任务书未写）
- 方案：已确认（`docs/方案.md`、`docs/安全审计.md`）
- 编码授权：T1 已获得；用户已另行授权 T1 commit，不含 push/发布。T2 附加范围待确认，尚未启动编码。
- 当前基线：本次首次提交为 T1 基线，具体 SHA 以 `git log -1` 为准

## 已完成内容

- 问答确认方案；只读审计 admin 底座安全问题，修复范围"高危全修 + 关键中危"
- T1 编码（Claude Code opus，会话 142d3cb7-f20d-4c53-bb0c-96716ce0a3d5）+ 窄修 1（同会话续接）
- 事故：执行器前序会话读取并复制了 admin `docs/account.md`（含真实凭证），已删除，未进入 git；测试文件曾写入凭证片段，窄修已删除；`.pi/evidence/` 与 /tmp 输出中的片段已由 Pi 替换为 `<REDACTED>`。全仓扫描 0 命中

## 本机候选验收（T1，窄修 1 之后）

- 编码 Agent 自报：vet/test 通过、三组合 build 通过、前端 build 通过
- Pi 独立：`gofmt -l` 空、`go vet` 通过、`go test ./...` 通过（generator 19s）
- 交互：expect 模拟真实终端（需应答 survey 的 `ESC[6n` 光标查询）走完 `taurus-x new demo-x`，全选组件生成成功；已存在目录在"输出目录"步骤即提示并可重输
- 生成产物：无 admin/内网残留；12 张底座表、74 条权限；go.mod 只含所选组件；compose `config` 通过（含 consul/etcd/minio/milvus）
- 运行：Docker `taurusx-e2e-mysql`(127.0.0.1:23306, mysql:8.0) / `taurusx-e2e-redis`(127.0.0.1:26379)；后端 127.0.0.1:19080，TCP 28081、gRPC 25051 同时监听；`/health` 200
- 浏览器（独立测试浏览器 isolatedContext）：admin 登录成功，标题为项目名，首页 + 9 个底座页面正常，新增部门写库成功，控制台无 error/warn
- SIGTERM：HTTP→gRPC→TCP→日志→Redis→DB 依次优雅关闭
- 未验证：consul/milvus/otel 的实际连接（无对应服务，仅 compose 配置校验）；Docker 镜像构建与 compose 整体启动
- 本机候选结论：**验收通过**
- 观察到的 T2 范围问题：访问日志记录登录明文密码与 JWT（审计 #2）；登录日志 0 条（已知缺陷）；pprof 6060 与本机 admin 冲突仅告警

## Git 状态

- 本次提交保存 T1；未配置 remote，未 push、未发布
- 本次仅核对文件、Git 状态及常见敏感文件名/密钥模式，未重跑测试；上文验收结果为 T1 上次验收记录

## 保留的测试资源（T2 复用，T2 结束后清理）

- 容器与卷：`taurusx-e2e-mysql`、`taurusx-e2e-redis`、卷 `taurusx-e2e-mysql-data`
- 目录：`/tmp/taurusx-e2e/`（含测试项目、测试 DB 密码文件 0600）、`/tmp/taurusx-bin/`、`/tmp/taurusx-test-*`、`/tmp/gocache-taurusx`、`/tmp/taurusx-ref/`（core 参考 clone）、`/tmp/taurusx-T1*.out|pid`

## 未解决问题 / T2 候选

- 审计 T2 范围（见 `docs/安全审计.md`）
- 执行器发现：初始管理员默认密码对所有项目固定；`.env.example` 写入了随机 JWT 密钥（进库文件）；`admin_depts.dept_code` 唯一键导致第二个空编码部门创建失败；普通角色未配接口权限时不能改自己密码；`vite.config.ts` 仍代理 `/downloads`、gitignore/.dockerignore 有 downloads 残留
- TCP 关闭时库打印 "server is already stopping"（不影响关闭结果）

## 下一步

- 确认 T2 附加范围后写 T2 任务书，再明确编码授权；T2 为安全关键任务，需独立复审及重新运行验收
- 下次续接可说：“继续 taurus-x 的 T2，先读 HANDOVER.md 并核对 Git 状态。”

## 执行器会话信息

- executor：claude；请求模型 opus；响应证明模型 claude-opus-5
- session_id：142d3cb7-f20d-4c53-bb0c-96716ce0a3d5（T1 专用；T2 为新任务需开新会话）
