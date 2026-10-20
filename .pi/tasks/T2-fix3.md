# T2 窄修 3：登录失败审计状态修正与过期文档同步

同一T2授权内实现缺陷修复，不扩大功能。基线4031f55，Claude Code opus原session aa1aa422-908c-4617-ac41-cfe573bd6a2c；不commit/push/发布，原保护边界全部保留。

## 已确认运行阻断

当前全选候选在独立 MySQL8.0/Redis7 真实运行：错误密码登录 body.code=401；登录日志有失败原因的12条记录却全部login_status=1；另13条无失败原因状态也为1。证据 /tmp/taurusx-t2-runtime/REPORT.md（不读同目录state.json或凭证）。Pi已读实际模板确认 model/admin_user_login_log.go.tmpl 的 LoginStatus int8 带 gorm default:1，RecordLogin失败传0后经普通Create，GORM将零值替为默认1。

## 修改要求

1. 修正LoginStatus的持久化契约：失败0必须显式持久化为0，成功1为1。优先移除模型中导致零值替换的default:1；若选其他方案须证明不会遗漏0。数据库初始化默认值可保留但必须保证ORM显式失败不会被覆盖。
2. 添加真正执行GORM Create路径的回归测试（可使用MySQL dialector DryRun + DisableAutomaticPing/SkipDefaultTransaction，不引入SQLite）。断言SQL绑定变量/模型结果均保留失败0和成功1。不能仅字符串查tag或只测RecordLogin参数。
3. 同步当前改动导致的过期生成文档：templates/project/docs/项目规范.md.tmpl 与 后端开发流程.md.tmpl 仍描述已删除PasswordChangeValidator和“两条链”。按实际routes.go更新为JWT+SessionValidator及三条链（基础读取、自助写入、受接口权限保护），保留BodyLimit/访问操作日志/CSRF的准确顺序，不猜测。
4. 更新允许范围README必要说明，不把候选写成已通过业务验收。

## Verify与限制

原T2三组合生成build/vet/test及全选前端install/build重跑。禁止改Pi维护的根docs/HANDOVER/.pi、不读取.env/.env.local/account/state.json等凭证，不改admin或模块缓存，不全局安装，不启服务Docker，不操作现有运行验收资源。不能靠删/放松测试过关。完成报告实际文件和命令结果。Pi将重新生成候选并真实验证成功/失败日志状态，旧候选验收不代替新候选验收。
