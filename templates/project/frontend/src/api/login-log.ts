// 登录日志接口（/admin/login-log/*）。
import { request } from './http'
import type { LoginLogListRequest, LoginLogListResponse } from '../types'

// 获取登录日志列表
export function apiGetLoginLogList(params: LoginLogListRequest) {
  return request<LoginLogListResponse>({ url: '/admin/login-log/list', method: 'post', data: params })
}
