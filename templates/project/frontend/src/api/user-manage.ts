// 用户管理接口（管理员视角，/admin/user/*）。
import { request } from './http'
import type {
  AddUserRequest,
  UpdateUserRequest,
  UserInfo,
  UserListRequest,
  UserListResponse,
  UserRoleDetailResponse,
} from '../types'

// 获取用户列表
export function apiGetUserList(params: UserListRequest) {
  return request<UserListResponse>({ url: '/admin/user/list', method: 'post', data: params })
}

// 获取用户详情（含角色权限）
export function apiGetUserInfo(user_id: number) {
  return request<{ user: UserInfo; permissions: UserRoleDetailResponse }>({
    url: '/admin/user/info',
    method: 'post',
    data: { user_id },
  })
}

// 新增用户
export function apiAddUser(data: AddUserRequest) {
  return request<{ status: string }>({ url: '/admin/user/add', method: 'post', data })
}

// 更新用户
export function apiUpdateUser(data: UpdateUserRequest) {
  return request<{ status: string }>({ url: '/admin/user/update', method: 'post', data })
}

// 更新用户状态
export function apiUpdateUserStatus(user_id: number, status: number) {
  return request<{ status: string }>({
    url: '/admin/user/update-status',
    method: 'post',
    data: { user_id, status },
  })
}

// 删除用户（软删）
export function apiDeleteUser(user_id: number) {
  return request<{ status: string }>({ url: '/admin/user/delete', method: 'post', data: { user_id } })
}
