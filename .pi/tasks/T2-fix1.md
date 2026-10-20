# T2 窄修 1：Pi 已复现的安全阻断

- 原任务、执行器与基线不变：T2 / Claude Code opus / `4031f55`，续接编码 session `aa1aa422-908c-4617-ac41-cfe573bd6a2c`。
- 原 T2 编码授权内的修复；不含 commit/push/发布；所有保护路径、凭证禁读、禁止长期服务及 Docker 约束继续生效。

## 真实证据

Pi 对当前生成的 `/tmp/taurusx-test-all/backend` 添加临时诊断测试，执行 `GOCACHE=/tmp/gocache-taurusx go test -count=1 ./app/helper/store ./internal/taurus -run '^TestPiDiagnostic'`，三用例均按预期失败，临时文件已删除：

1. RegisterSession → InvalidateUserSessions → 删除 epoch 键（保留 jti）→ ValidateSession 返回 nil，已吊销会话复活。
2. RegisterSession → KV 故障 → InvalidateUserSessions 返回 error → KV 恢复 → ValidateSession 返回 nil。结合 `auth_service.ChangePassword/SetPassword/UpdateUserPassword` 与 `user_service.UpdateUserStatus` 先提交 MySQL 再吊销，可导致密码/状态已改变而旧会话恢复有效。
3. NewDailyGormLogger(..., logger.Warn) 的 Trace 错误分支日志包含回调 SQL 的合成敏感值。`daily_log.go.tmpl:119-136` 错误/慢 SQL 均写带参 SQL；仅改默认 warn 不满足原任务书“数据库日志避免输出敏感 SQL 参数”。

## 必须修复

1. `internal/taurus/daily_log.go.tmpl`：不再将插值 SQL 或含敏感值的原始数据库错误写日志。保留耗时/行数/安全错误分类等排障信息即可；即使设置 info 也不得泄漏。增加 logger 行为测试覆盖 error/slow/info 与敏感合成 marker 不出现。
2. `app/helper/store/auth_redis_store.go.tmpl`：缺少吊销版本不得默认回退成 0 后复活已有会话。不要把“加 TTL”当成解决淘汰的办法，TTL 之前也可能被 allkeys 淘汰。随机版本/缺失拒绝等安全初始化须保持首次登录可用，需补缺失版本测试。
3. `auth_service.go.tmpl`、`user_service.go.tmpl` 及签发/刷新调用链：使凭证或状态变更与会话安全边界一致；不得有 MySQL 已改、Redis 吊销失败恢复后旧 Token 有效的状态。可用数据库持久化认证版本并同事务变更，或采用能证明正确的等价方案；若需新增底座列，限初始化 SQL 与模型同步（不改 admin，不做生产迁移）。还须消除 PasswordLogin 校验后到 RegisterSession 读取新版本之间的竞态，以及旧刷新校验通过后跨过改密/禁用再签发新版本的竞态。不要仅把 Incr 调到 SQL 前面就宣称解决：旧凭证并发登录仍可能登记到新版本。补确定性行为回归测试，不用 sleep 依赖调度。
4. `app/bootstrap.go.tmpl`：后台 JWT 签名功能实际仍使用时，不能通过 `http.jwt.enabled=false` 跳过密钥自检；去掉该跳过分支，补调用契约验证。
5. `auth_controller.go.tmpl` 提取 Token 与 `middleware.GetJWTToken` 优先级统一，controller 直接复用，确保登出吊销的是请求认证使用的 Token；补同时带不同 Cookie/Bearer 的行为用例。
6. 原 T1 已要求删除的未路由 Register controller/service 死代码仍残留，删除仅注册使用的 DTO/调用（先查引用，不破坏新增用户流程）。

## 复审纠偏

正式 review 证据 `.pi/evidence/T2-review.json` session `0b00f4be-e48b-48d0-8cb8-a6c7b91058f5`。其“无阻断”结论不采纳：SQL 敏感参数在任务范围内；epoch 淘汰违反吊销契约；其“同秒新登录只在注释”的观察也不准确，已有 `TestSession_InvalidateAllThenRelogin` 真正执行该场景。以上均以实际代码和 Pi 诊断为准。

其他未确认范围不扩做（手机号自助、权限 is_system、Cookie 统一、bcrypt、限流公共库替换等）。README 不得把未通过验收的修复宣称已验证，纠正 SQL 日志已安全等过度结论，部门编码是创建后不可修改，不应称编辑缺陷。

## Verify

原 T2 全部测试要求保留；增加上述行为测试，三组合生成后 build/vet/test，全选前端 install/build。Pi 将重新独立验证；旧自动化通过结果不能证明窄修后的候选。独立复审必须新会话。执行器失败停下报告，不重试或切换。
