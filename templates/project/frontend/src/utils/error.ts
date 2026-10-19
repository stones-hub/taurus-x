// 从 catch 捕获的 error 对象中提取可展示的错误信息。
// 统一各页面的 try/catch 错误提示，避免每个文件重复定义。
export function extractMessage(e: unknown, fallback: string): string {
  if (e && typeof e === 'object' && 'message' in e) {
    const msg = (e as { message?: unknown }).message
    if (typeof msg === 'string' && msg) return msg
  }
  return fallback
}