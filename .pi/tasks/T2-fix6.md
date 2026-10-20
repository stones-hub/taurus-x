# T2 窄修6：资料编辑的角色更新语义

用户明确选择并授权一并修复：两处错误赋角路径；role_ids 未传=不改、显式[]=清空；编辑页面保留已有禁用角色关联，补行为测试。原Claude Code opus session aa1aa422-908c-4617-ac41-cfe573bd6a2c，基线4031f55。不commit/push/发布；原授权和保护边界不变。

## 证据

独立复审 .pi/evidence/T2-review-fix5.json；Pi已rg确认：user_service.UpdateUser无条件replaceUserRolesTx；README:262、controller/user_admin_guard_test.go.tmpl:105错写/admin/user/assign-roles，真实路由/admin/role/assign。

## 要求

1. UpdateUser DTO显式区分字段缺失与空数组，保持API兼容：未传role_ids不校验/替换角色关联，显式[]清空（仍受系统角色及超管目标保护）；非空数组按原安全校验替换。null须明确安全语义并测试，推荐与未传一致，不意外清空。不要让AddUser或专用assign端点的既有语义意外改变。
2. UpdateUser同事务身份锁和超管目标保护仍必须执行，与是否更新角色无关；省略role_ids不能绕过账号保护或auth_version原子变更。选择性角色写入放在预检后；拒绝零副作用。
3. 编辑页面显示/携带目标当前持有的禁用角色，不能因仅加载启用角色而在保存资料时丢失关联。禁止趁机允许新增已禁用角色：已有禁用关联可保留/按明确操作移除，新增禁用关联必须拒绝。返回选项及服务校验以真实DB事实为准，不信前端。优先最小改法，保留一处编辑入口，不新增页面配置。
4. 纠正README与controller测试注释的错误路由为/admin/role/assign；全templates检查无错误assign-roles残留。
5. 行为回归覆盖handler JSON解析→service→repo：省略不删、不传资料保持所有关联；显式[]删除普通角色；null明确行为；保留原禁用角色成功但新增禁用角色拒绝；超管拒绝零写；管理重置密码/锁序/auth_version不回归。禁用角色前端回显及提交类型检查通过。不能仅源码字符串断言。
6. 同步模板README/项目规范与API语义说明，不把代码/单测当真库验收。原T2测试全套vet/test、三组合生成build/vet/test和全选前端指定pnpm install/build。

## 边界

禁止重试/绕过之前安全拦截的运行诊断，不生成执行攻击复现。本轮仅防御性数据正确性修复和常规测试。只改internal/templates/README及必要依赖；不改Pi根docs/HANDOVER/.pi；禁读.env/.env.local/account/state.json和凭证；不动admin模块缓存及运行验收现场，不全局安装、不起Docker长期服务。失败停下不自动重试/换执行器，不删除放松测试。最终报告真实文件/命令/剩余风险。
