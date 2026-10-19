// 当前用户（个人信息）接口：/admin/user/*。
import { request } from './http'
import type {
  BindMobileRequest,
  ChangePasswordRequest,
  SendCodeRequest,
  SetPasswordRequest,
  UnbindMobileRequest,
  UpdateProfileRequest,
  UserInfo,
} from '../types'

// 获取当前用户信息
export function apiCurrentInfo() {
  return request<UserInfo>({ url: '/admin/user/current-info', method: 'get' })
}

// 更新个人信息
export function apiUpdateProfile(data: UpdateProfileRequest) {
  return request<{ status: string }>({ url: '/admin/user/update-profile', method: 'post', data })
}

// 修改密码
export function apiChangePassword(data: ChangePasswordRequest) {
  return request<{ status: string }>({ url: '/admin/user/change-password', method: 'post', data })
}

// 设置/重置密码
export function apiSetPassword(data: SetPasswordRequest) {
  return request<{ status: string }>({ url: '/admin/user/set-password', method: 'post', data })
}

// 绑定手机号
export function apiBindMobile(data: BindMobileRequest) {
  return request<{ status: string }>({ url: '/admin/user/bind-mobile', method: 'post', data })
}

// 解绑手机号
export function apiUnbindMobile(data: UnbindMobileRequest) {
  return request<{ status: string }>({ url: '/admin/user/unbind-mobile', method: 'post', data })
}

// 发送验证码
export function apiSendCode(data: SendCodeRequest) {
  return request<{ status: string }>({ url: '/admin/user/send-code', method: 'post', data })
}
