// 认证相关接口。
import { request } from './http'
import type { LoginRequest, LoginResponse, UserInfo } from '../types'

export function apiLogin(data: LoginRequest) {
  return request<LoginResponse>({ url: '/auth/login', method: 'post', data })
}

// 当前用户信息（/admin/user/current-info）
export function apiMe() {
  return request<UserInfo>({ url: '/admin/user/current-info', method: 'get' })
}
