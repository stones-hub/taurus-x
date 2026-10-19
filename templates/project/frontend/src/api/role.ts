// 角色管理接口（/admin/role/*）。
import { request } from './http'
import type {
  AddRoleRequest,
  EditRoleInfoResponse,
  RoleListRequest,
  RoleListResponse,
  UpdateRoleRequest,
} from '../types'

// 获取角色列表
export function apiGetRoleList(params: RoleListRequest) {
  return request<RoleListResponse>({ url: '/admin/role/list', method: 'post', data: params })
}

// 新增角色
export function apiAddRole(data: AddRoleRequest) {
  return request<{ status: string }>({ url: '/admin/role/add', method: 'post', data })
}

// 更新角色
export function apiUpdateRole(data: UpdateRoleRequest) {
  return request<{ status: string }>({ url: '/admin/role/update', method: 'post', data })
}

// 删除角色
export function apiDeleteRole(role_id: number) {
  return request<{ status: string }>({ url: '/admin/role/delete', method: 'post', data: { role_id } })
}

// 更新角色状态
export function apiUpdateRoleStatus(role_id: number, status: number) {
  return request<{ status: string }>({
    url: '/admin/role/update-status',
    method: 'post',
    data: { role_id, status },
  })
}

// 获取编辑角色信息（含全量权限树与选中状态）
export function apiGetEditRoleInfo(role_id: number) {
  return request<EditRoleInfoResponse>({
    url: '/admin/role/edit-info',
    method: 'post',
    data: { role_id },
  })
}
