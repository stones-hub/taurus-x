// 改密成功后的收尾：后端已吊销该用户全部会话，当前令牌不再可用。
// 立即清本地登录态并带原因回登录页（登录页按 reason 显示提示），
// 不要停在原页面等下一次请求才报「会话已失效」。
//
// 依赖通过参数注入（清理函数、路由跳转），便于在不引入 Vue 组件测试依赖的情况下单测。
import { loginPath } from './login-reason'

export async function reloginAfterPasswordChange(deps: {
  clear: () => void
  navigate: (path: string) => unknown
}) {
  deps.clear()
  await deps.navigate(loginPath('password_changed'))
}
