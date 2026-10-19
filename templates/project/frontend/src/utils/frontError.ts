// 前端全局错误捕获上报 SDK
// 挂载 window.onerror + unhandledrejection，捕获未处理的 JS 错误与 Promise rejection，
// 上报到后端 /report-front-error（公共路由，无需登录）。
// 用独立 axios 实例（不走业务 instance，避免错误弹窗循环 + 不受 JWT 拦截干扰）。
import axios from 'axios'
import { baseURL } from '../api/http'

// 独立 axios 实例：仅用于错误上报，超时短、不重试、静默失败
const reportInstance = axios.create({
  baseURL,
  timeout: 5000,
  withCredentials: true,
})

interface FrontErrorReport {
  message: string
  source: string
  stack: string
  url: string
  kind: string
}

/** 上报错误（静默，失败不影响前端） */
function report(payload: FrontErrorReport) {
  reportInstance
    .post('/report-front-error', payload)
    .catch(() => {
      // 上报失败静默忽略，避免错误上报本身再引发错误循环
    })
}

/** 截断过长字符串 */
function truncate(s: string, max: number): string {
  if (!s) return ''
  return s.length > max ? s.slice(0, max) : s
}

/** 安装全局错误捕获 */
export function setupFrontErrorReporting() {
  // 1. 同步运行时错误
  window.addEventListener('error', (event) => {
    const target = event.target
    // 忽略资源加载错误（img/script/link 等），只关注 JS 错误
    if (target && (target as HTMLElement).tagName && !(event instanceof ErrorEvent)) {
      return
    }
    const e = event as ErrorEvent
    report({
      message: truncate(e.message || 'Unknown error', 1000),
      source: truncate(e.filename ? `${e.filename}:${e.lineno}:${e.colno}` : '', 500),
      stack: truncate(e.error?.stack ?? '', 8000),
      url: window.location.href,
      kind: 'error',
    })
  })

  // 2. 未处理的 Promise rejection
  window.addEventListener('unhandledrejection', (event) => {
    const reason = event.reason
    let message = 'Unhandled promise rejection'
    let stack = ''
    if (reason instanceof Error) {
      message = reason.message || message
      stack = reason.stack || ''
    } else if (typeof reason === 'string') {
      message = reason
    } else {
      try {
        message = JSON.stringify(reason)
      } catch {
        message = 'Unhandled promise rejection'
      }
    }
    report({
      message: truncate(message, 1000),
      source: '',
      stack: truncate(stack, 8000),
      url: window.location.href,
      kind: 'unhandledrejection',
    })
  })
}
