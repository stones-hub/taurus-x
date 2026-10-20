# T2 收口：非阻断精度修正与验收

用户明确要求继续，按上一轮已列剩余范围收口，不扩大功能。Claude Code opus原session aa1aa422-908c-4617-ac41-cfe573bd6a2c，基线4031f55；不commit/push/发布。

## 编码范围（仅三点）

1. templates/project/backend/app/controller/user_permission_scope_test.go.tmpl中错误路由/admin/role/menus-buttons纠正为/admin/user/menus-buttons，新增生成产物禁止错误路径断言。
2. role_service及测试说明改为：有效叶子权限由过滤启用角色/权限的统一出口取得；祖先只为已授权后代补全且必须启用。不能声称完全无第二次补查。
3. 补parent_id非0的嵌套菜单行为测试：有效子菜单补出启用祖先；没有有效后代时禁用角色不能借祖先补查让菜单出现；如适用测试禁用祖先不补出。走现有真实service/repo链并明确sqlmock不证明真库执行。不得改产品权限语义或扩修其他风险。

原vet/test-count1、三组合生成build/vet/test、全选前端指定pnpm install/build，最终明确完成或受阻报告。

禁止读.env/.env.local/account/state.json或凭证，不改Pi根docs/HANDOVER/.pi、admin模块缓存、不动现有运行验收现场、不起Docker长期服务、不全局安装、不commit/push。此前被拦截并发运行诊断不重试、不绕过、不生成执行。失败停止不换执行器。

## Pi后续验收范围

在独立临时目录重建当前候选，使用本机MySQL8/Redis7合成测试数据，常规验证禁用角色→启用恢复、多角色并集、资料编辑保留禁用角色与选择性更新、个人中心入口及改密、登录日志状态和底座页面。使用独立测试浏览器，完成后清理本次测试资源。真实并发交错仍未验证，不冒称完成。若受限立即停止报告，不绕过限制。
