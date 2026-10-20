# T2 窄修4：超管账号保护与个人中心入口

## 授权与执行顺序

用户已明确确认并授权：非超管不得修改、重置密码、禁用或删除超管账号；权限检查须在任何写入前完成，拒绝不得留下部分修改；补齐个人中心可点击入口，优先复用头像/用户名，不新增页面。

必须等窄修3返回且核验结果后，再串行续接原Claude Code opus会话 aa1aa422-908c-4617-ac41-cfe573bd6a2c，基线4031f55。不并发写入，不commit/push/发布；原T2保护边界不变。

## 已确认事实

独立复审 `.pi/evidence/T2-review-fix2.json`：controller/user_controller.go.tmpl UpdateUser先UserService.UpdateUser提交资料，再AuthService.UpdateUserPassword提交密码，最后RoleService.UpdateUserRoles做系统角色保护。Pi已读实际控制器确认顺序；非超管有接口权限时可重置超管密码，后续403也不撤销密码更新。个人中心虽有profile路由，但复审未找到顶栏可点击入口，需实际检查后补齐。

## 范围与契约

1. 对用户管理写入建立统一目标账号保护：持有普通user/update、update-status、delete等接口权限不等于可管理超管账号。操作人必须来自JWT，非超管不得对拥有super_admin角色的目标执行资料更新、密码重置、禁用、删除；不能信任入参传入的角色列表或is_system来判定目标身份。身份查询失败必须拒绝，不能放行。
2. 所有身份/角色权限预检在任何资料/凭证/角色写入前完成。UpdateUser后段角色保护拒绝不得留下前段资料/密码已改的半成功状态；必要时把本次用户编辑涉及的写入组织在同一事务内，在事务内验证目标与操作人事实（考虑角色变更并发边界），不拆成多个独立提交后才检查。
3. 保留合法超管管理普通账号以及普通管理员管理普通账号的既有授权功能。本人自助change-password仍须旧密码且不受用户管理权限阻塞；不要误把超管本人改密挡住。非超管不得借本人资料/角色操作抬升为super_admin。
4. 对UpdateUser、UpdateUserStatus、DeleteUser、管理重置的实际调用链补行为回归测试：非超管管理超管拒绝且零写入/版本不变、同角色集合也拒绝、带空/变更role_ids拒绝无副作用、查询错误fail closed；合法管理普通用户及本人自助改密正常。不能仅测纯守卫而不查真实布线。
5. 个人中心复用既有页面：顶栏用户名/头像可点击进入现有profile路由，支持正常键盘操作，无新增菜单权限或新页面的必要就不加。修正必要描述，前端build与浏览器验证入口及改密流程。
6. 同步相关文档，尤其后端开发流程“option复用业务list handler”过期安全指引必须改成最小字段专用handler（如窄修3未处理）；不改无关功能。

## Verify

原T2全套vet/test/三组合build/vet/test与前端install/build，独立复审新会话和Pi真实HTTP权限场景、MySQL副作用核对、浏览器个人中心点击与改密。不删除或放松测试。

边界：只修改本仓获准internal/templates/README/必要依赖；根docs/HANDOVER/.pi由Pi维护；禁读真实或测试凭证/.env/.env.local/account/state.json；不动admin模块缓存、不全局安装、不操作运行验收现场。执行器失败停止，不重试/不切换。
