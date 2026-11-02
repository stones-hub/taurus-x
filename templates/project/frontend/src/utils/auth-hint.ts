// 浏览器端登录态的「提示」：不碰令牌本身。
//
// JWT 只在 HttpOnly Cookie 里，前端 JS 读不到，也不应该读到。路由守卫要一个
// 「看起来已登录」的判断，就看非 HttpOnly 的 CSRF Cookie 在不在：后端登录时把它和
// JWT Cookie 一起下发、有效期相同，刷新时一起续期，退出时一起清掉。它本身不是凭证
// （伪造一个也过不了后端认证），所以这只是体验上的判断，是否真的登录以服务端 401 为准。

/** CSRF Cookie 名（与后端 middleware.CSRFTokenKey 一致） */
export const CSRF_COOKIE = 'x-csrf-token'

/** 旧版本把令牌存在 localStorage 的键名，只用于启动时清理。 */
const LEGACY_TOKEN_KEY = 'token'

/** 从 cookie 串里读取指定 key；默认读 document.cookie。 */
export function readCookie(name: string, cookies: string = document.cookie): string {
  const m = cookies.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return m ? decodeURIComponent(m[1]) : ''
}

/** 是否「看起来已登录」：CSRF Cookie 存在。 */
export function hasSessionHint(cookies?: string): boolean {
  return readCookie(CSRF_COOKIE, cookies) !== ''
}

/**
 * 清掉登录提示（改密、退出、会话失效时调用）：删除 CSRF Cookie。
 * HttpOnly 的 JWT Cookie 前端删不掉；它要么已被后端吊销，要么由退出接口清除。
 */
export function clearSessionHint(doc: { cookie: string } = document) {
  doc.cookie = `${CSRF_COOKIE}=; Max-Age=0; path=/`
}

/** 删掉旧版本留在 localStorage 里的令牌（应用启动时调用一次）。 */
export function purgeLegacyToken(storage: Pick<Storage, 'removeItem'> = localStorage) {
  storage.removeItem(LEGACY_TOKEN_KEY)
}
