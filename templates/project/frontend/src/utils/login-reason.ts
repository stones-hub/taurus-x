// 整页 / 路由跳转到登录页时携带的原因（/login?reason=expired）。
//
// 只接受下面固定的几个值：reason 来自 URL，任何人都能构造链接，
// 不能把参数原文显示出来，也不能随便什么值都弹提示。

/** 原因值 → 登录页提示文案 */
export const LOGIN_REASONS = {
  expired: '登录已过期，请重新登录',
  password_changed: '密码已修改，请重新登录',
} as const

export type LoginReason = keyof typeof LOGIN_REASONS

/** 生成带原因的登录页地址。 */
export function loginPath(reason: LoginReason): string {
  return `/login?reason=${reason}`
}

/**
 * 读取并消费地址栏里的 reason：返回对应提示（不认识的值返回 null），
 * 只要带了 reason 参数就用 history.replaceState 清掉，刷新页面不会重复提示。
 * 其他查询参数原样保留。
 */
export function takeLoginReason(
  location: { pathname: string; search: string; hash: string },
  history: Pick<History, 'replaceState' | 'state'>,
): string | null {
  const params = new URLSearchParams(location.search)
  if (!params.has('reason')) return null
  const reason = params.get('reason') ?? ''
  params.delete('reason')
  const query = params.toString()
  history.replaceState(history.state, '', location.pathname + (query ? `?${query}` : '') + location.hash)
  return Object.prototype.hasOwnProperty.call(LOGIN_REASONS, reason) ? LOGIN_REASONS[reason as LoginReason] : null
}
