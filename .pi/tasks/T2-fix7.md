# T2 窄修7：禁用角色不再贡献权限，补齐收口测试与文档

用户明确选择授权：过滤禁用角色的接口/菜单/按钮权限，补调用链测试及禁用启用回归；补明确移除已有禁用角色测试；注明资料仍按完整表单更新。不扩其他功能。原Claude Code opus session aa1aa422-908c-4617-ac41-cfe573bd6a2c，基线4031f55，不commit/push/发布。

## 证据

独立复审.pi/evidence/T2-review-fix6.json，Pi读repo/admin_user_role_repo.go.tmpl确认FindUserPermissions只有ur→rp→p联表且仅p.status=1，无角色状态过滤。窄修6回归未覆盖已有禁用角色明确移除。userProfileUpdates仍整表单语义，不能说端点整体PATCH。

## 要求

1. FindUserPermissions及接口/菜单/按钮实际权限链仅由启用角色和启用权限贡献；追查所有相关查询路径统一，不能只改菜单接口。super_admin判定仍要求启用，系统角色不可被普通管理接口禁用等保护不回归。不引入权限缓存或新系统。
2. 禁用角色时保留用户关联用于恢复/回显，但不贡献权限；启用后恢复权限（仍受其权限行状态控制）。一个用户多个角色时集合并集正确：禁用角色不能贡献独占权限，其他启用角色共享权限仍有效。身份查询错误fail closed。
3. 加真实handler/service/repo/GORM路径测试与MySQL dialector SQL断言，验证过滤实际生效的查询形状及接口checker行为、菜单/按钮来源，覆盖禁用/启用/多角色/权限本身禁用。sqlmock局限明确，不冒充真库。
4. 补窄修6遗漏：目标当前持有普通禁用角色，明确提交不含它的新集合，成功提交且移除关联；加入CheckRoleStatusAssignable保留/取消/新增拒绝测试（超管也不能新增禁用角色）。保留超管保护零写及选择性角色语义。
5. 文档明确：仅role_ids未传/null不改、[]清空，资料其他字段仍完整表单语义；只改角色请走已有/admin/role/assign，不能宣称整个UpdateUser都是PATCH。修正禁用角色语义为不贡献接口/菜单/按钮权限，关联可保留可移除。不要扩大到指针式整表单改造。
6. 重跑原T2 go vet/test-count1、三组合生成build/vet/test及全选前端指定pnpm install/build；完整报告实际变更/命令结果/未验证项。不删除放松断言。

## 边界

只改internal/templates/README和必要依赖，不改Pi根docs/HANDOVER/.pi，禁读.env/.env.local/account/state.json或任何凭证，不动admin模块缓存及运行现场，不全局安装、不启Docker长期服务。此前被拦截运行诊断不重试/绕过，不编写执行攻击复现；仅防御性鉴权修复和常规测试，受限停止报告不换执行器。最终真库及浏览器仍由Pi按可验证范围验收。
