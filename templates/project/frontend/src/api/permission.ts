// 权限 / 菜单接口。
import { request } from './http'
import type {
  AddPermissionRequest,
  EditPermissionInfoResponse,
  PermissionListRequest,
  PermissionListResponse,
  PermissionTreeRequest,
  PermissionTreeNode,
  UpdatePermissionRequest,
  UserMenuButtonResponse,
} from '../types'

// 当前用户菜单树 + 按钮权限（/admin/user/menus-buttons）
export function apiGetMenuButtons() {
  return request<UserMenuButtonResponse>({ url: '/admin/user/menus-buttons', method: 'get' })
}

// 获取权限列表
export function apiGetPermissionList(params: PermissionListRequest) {
  return request<PermissionListResponse>({ url: '/admin/permission/list', method: 'post', data: params })
}

// 获取权限树（可选按类型过滤父权限）
export function apiGetPermissionTree(params: PermissionTreeRequest = { parent_id: 0, permission_type: 0 }) {
  return request<{ tree: PermissionTreeNode[] }>({ url: '/admin/permission/tree', method: 'post', data: params })
}

// 新增权限
export function apiAddPermission(data: AddPermissionRequest) {
  return request<{ status: string }>({ url: '/admin/permission/add', method: 'post', data })
}

// 更新权限
export function apiUpdatePermission(data: UpdatePermissionRequest) {
  return request<{ status: string }>({ url: '/admin/permission/update', method: 'post', data })
}

// 删除权限
export function apiDeletePermission(permission_id: number) {
  return request<{ status: string }>({ url: '/admin/permission/delete', method: 'post', data: { permission_id } })
}

// 更新权限状态
export function apiUpdatePermissionStatus(permission_id: number, status: number) {
  return request<{ status: string }>({
    url: '/admin/permission/update-status',
    method: 'post',
    data: { permission_id, status },
  })
}

// 更新是否系统权限
export function apiUpdatePermissionIsSystem(permission_id: number, is_system: number) {
  return request<{ status: string }>({
    url: '/admin/permission/update-is-system',
    method: 'post',
    data: { permission_id, is_system },
  })
}

// 获取编辑权限信息（含权限树）
export function apiGetEditPermissionInfo(permission_id: number) {
  return request<EditPermissionInfoResponse>({
    url: '/admin/permission/edit-info',
    method: 'post',
    data: { permission_id },
  })
}
