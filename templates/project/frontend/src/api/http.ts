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

// 响应拦截器：兜底处理网络层 / 框架层真实非 2xx（如 429、真实 401）
instance.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    const msg = error.response?.data?.message || '请求失败'
    if (status === 401) {
      localStorage.removeItem('token')
      ElMessage.error('登录已过期，请重新登录')
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    } else {
      ElMessage.error(msg)
    }
    return Promise.reject(error)
  },
)

/** 提取业务错误信息（优先 data.message，其次 body.message）。 */
function extractError(body: ApiResponse): string {
  const data = body.data
  if (typeof data === 'string' && data) {
    return data
  }
  if (data && typeof data === 'object' && typeof (data as Record<string, unknown>).message === 'string') {
    return (data as Record<string, string>).message
  }
  return body.message || '请求失败'
}

/** 发起请求并解包 data；按 body.code 判定业务成败（2xx 视为成功）。 */
export async function request<T>(config: AxiosRequestConfig): Promise<T> {
  const res = await instance.request<ApiResponse<T>>(config)
  const body = res.data
  if (body.code < 200 || body.code >= 300) {
    const msg = extractError(body)
    if (body.code === 401) {
      localStorage.removeItem('token')
    }
    ElMessage.error(msg)
    throw new Error(msg)
  }
  return body.data
}

export default instance
