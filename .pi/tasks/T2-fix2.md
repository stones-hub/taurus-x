# T2 窄修 2：删除重复的无旧密码自助改密入口

## 授权与基线

用户已明确选择并授权：删除 set-password 接口、权限行和对应重复页面/入口，个人中心统一使用已有的旧密码+新密码改密流程；保留管理员用户管理重置他人密码功能。允许按此窄修后重新测试、独立复审和本机验收。不含 commit/push/发布。

同一 T2 任务，同一 Claude Code opus 编码会话 `aa1aa422-908c-4617-ac41-cfe573bd6a2c`，Git 基线仍为 `4031f55`。原任务全部保护边界不变。

## 已确认事实与具体改法

Pi 已读 `templates/project/backend/app/controller/user_controller.go.tmpl:74-90`：SetPassword 从 JWT 取本人 ID，DTO 只收 new_password，调用 AuthService.SetPassword 不校验旧密码。README 和 generator 测试“管理员重置他人”的描述错误。复审证据 `.pi/evidence/T2-review-fix1.json`。

1. 删除后端 `/admin/user/set-password` 路由、controller SetPassword、service SetPassword、仅其使用的 DTO 与引用。保留 ChangePassword 的旧密码校验、事务变更和版本递增；保留 UpdateUserPassword（管理员重置他人）。不要为了删除重复入口顺带改管理员权限模型。
2. 删除前端 profile/set-password.vue、专用路由/API/type 与对应入口（查全引用）；个人中心原有旧密码+新密码流程保留可用，用户点击改密必须进入该流程，不能留下死链接。
3. 删除初始化 SQL 的该接口权限行及仅该重复入口使用的菜单/按钮关联（如有）；核对实际权限总数和分类数，README 不使用猜测数字。
4. 改正 generator 测试：不再固化 set-password 管理他人的错误假设，断言旧入口不存在、普通用户 change-password 正常受 JWT/会话/CSRF 保护且无需用户管理权限、管理员 user/update 仍需权限。修改必要行为测试，不能通过删除安全断言降低验证标准。
5. 更新根 README、模板 README，说明自行改密必须旧密码、管理员重置仍在用户管理；移除旧描述。

## 不扩做

本次不自动纳入复审 A2 AddUser 事务、A3 logger Info/Warn/Error 或 A4 未使用 Save，除非删除入口必须调整其引用。保留已知风险，不宣称已验证。其他原定不做范围不变。

## Verify

完整 `go vet ./... && go test -count=1 ./...`，三组合生成后 build/vet/test；全选前端指定 pnpm install/build；引用检查确保无失效页面/权限/接口调用。完成报告实际改动和命令结果，失败停止不重试、不切换。

Pi 后续：完整差异复核、独立重跑、新会话复审；真实 MySQL 8.0 + Redis 7 验证凭证变更与版本递增同事务（通过可控故障让第二步失败并确认回滚，不采用删除用户后仍试登录的无效设计）、旧Token失效、故障恢复，浏览器验收本人改密与管理他人重置。所有验收只在独立测试环境进行。
