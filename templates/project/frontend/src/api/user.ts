// 当前用户（个人信息）接口：/admin/user/*。
import { request } from './http'
import type {
  BindMobileRequest,
  ChangePasswordRequest,
  SendCodeRequest,
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

// 修改密码（自助改密唯一入口，必须带旧密码；
// 管理员重置他人密码走用户管理的 apiUpdateUser，带 password 字段）
export function apiChangePassword(data: ChangePasswordRequest) {
  return request<{ status: string }>({ url: '/admin/user/change-password', method: 'post', data })
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
