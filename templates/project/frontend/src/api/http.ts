// 统一的 axios 请求封装：自动携带 JWT + CSRF、统一解包 {code, message, data}。
// 约定：后端用 httpx.SendResponse 直接以 HTTP 状态码作为业务 code（成功=200），
// 且对 4xx/5xx 错误恒返回 HTTP 200 + body.code=错误码，因此这里按 body.code 判定成败。
import axios, { type AxiosInstance, type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import type { ApiResponse } from '../types'

// API baseURL（导出供 frontError 等独立 axios 实例复用，避免重复 env 解析）
export const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

// CSRF token key（与后端 middleware.CSRFTokenKey 一致）
const CSRF_KEY = 'x-csrf-token'

const instance: AxiosInstance = axios.create({
  baseURL,
  timeout: 15000,
  withCredentials: true, // 携带 cookie（JWT / CSRF 可走 cookie）
})

/**
 * 接口**已经给出响应**、只是业务 code 不是 2xx 时抛出的错误。
 *
 * 之所以要一个专门的类型，而不是继续抛裸 Error：这类失败服务端自己已经知道、
 * 用户也已经看到提示，不该再被 utils/frontError.ts 当成「前端 JS 异常」上报一遍。
 * （加了后台限流之后，快速切菜单撞上 429 会让它一轮往日志表里灌几十条
 * "Request failed with status code 429" —— 噪音挤掉真正的前端错误。）
 * 真实的 JS 运行时错误不是 ApiError，照旧上报。
 *
 * 仍然是 Error 的子类，所以 utils/error.ts 的 extractMessage 照常能取到 message。
 */
export class ApiError extends Error {
  /** 业务 code（后端用 HTTP 语义码作为业务码，如 401/403/500） */
  readonly code: number

  constructor(message: string, code: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

/**
 * 弹错误提示。grouping: true 开启 Element Plus 的同内容合并。
 *
 * 不合并的话，一次快速切换菜单（并发十几个列表请求）撞上限流会叠出二十多条
 * 一模一样的「请求过于频繁」，把整个屏幕糊住 —— 信息量和一条完全一样。
 */
function showError(message: string) {
  ElMessage({ message, type: 'error', grouping: true })
}

/** 从 cookie 读取指定 key（CSRF token 存于 cookie，HttpOnly=false）。 */
function getCookie(name: string): string {
  const m = document.cookie.match(new RegExp('(?:^|; )' + name + '=([^;]*)'))
  return m ? decodeURIComponent(m[1]) : ''
}

// 请求拦截器：附加 Bearer Token + CSRF header（非 GET）
instance.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  const method = (config.method ?? 'get').toUpperCase()
  if (method !== 'GET') {
    const csrf = getCookie(CSRF_KEY)
    if (csrf) {
      config.headers[CSRF_KEY] = csrf
    }
  }
  return config
})

/**
 * 提取错误提示：data 为非空字符串优先，其次 data.message，再其次 body.message。
 *
 * 这个优先级是跟着后端的 httpx.SendResponse 来的：它的 message 由公共库按状态码
 * 固定（429 固定是英文 "Too Many Requests"），只有 data 是业务能决定的，
 * 所以中文提示一律放在 data 里（见后端 middleware.rateLimitedMessage）。
 * 先读 message 的写法会把中文提示盖成英文，或者直接拿不到提示。
 */
function extractError(body: ApiResponse | undefined): string {
  const data = body?.data
  if (typeof data === 'string' && data) {
    return data
  }
  if (data && typeof data === 'object' && typeof (data as Record<string, unknown>).message === 'string') {
    return (data as Record<string, string>).message
  }
  return body?.message || '请求失败'
}

// 响应拦截器：兜底处理网络层 / 框架层真实非 2xx（如 429、真实 401）
instance.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    // 与业务错误走同一套取值优先级：真实非 2xx（429 等）的中文提示在 data 里，
    // 只读 data.message 的话用户看到的永远是兜底的「请求失败」。
    const msg = extractError(error.response?.data as ApiResponse | undefined)
    if (status === 401) {
      localStorage.removeItem('token')
      ElMessage({ message: '登录已过期，请重新登录', type: 'error', grouping: true })
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    } else {
      showError(msg)
    }
    return Promise.reject(error)
  },
)

/** 发起请求并解包 data；按 body.code 判定业务成败（2xx 视为成功）。 */
export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const res = await instance.request<ApiResponse<T>>(config)
  const body = res.data
  if (body.code < 200 || body.code >= 300) {
    const msg = extractError(body)
    if (body.code === 401) {
      localStorage.removeItem('token')
    }
    showError(msg)
    // 抛 ApiError 而不是裸 Error：调用方的 catch 行为不变（message 照旧），
    // 但 frontError.ts 能据此认出「这是接口失败，不是前端异常」，不再上报。
    throw new ApiError(msg, body.code)
  }
  return body.data
}

export default instance
