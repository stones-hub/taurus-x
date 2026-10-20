// 部门接口（/admin/dept/* 与 /admin/option/depts）。
import { request } from './http'
import type {
  AddDeptRequest,
  BatchUpdateDeptUserRequest,
  DeptListRequest,
  DeptListResponse,
  DeptOptionItem,
  EditDeptInfoResponse,
  UpdateDeptRequest,
  UserOptionListResponse,
} from '../types'

// 获取部门列表
export function apiGetDeptList(params: DeptListRequest) {
  return request<DeptListResponse>({ url: '/admin/dept/list', method: 'post', data: params })
}

// 获取编辑部门信息
export function apiGetEditDeptInfo(dept_id: number) {
  return request<EditDeptInfoResponse>({ url: '/admin/dept/edit-info', method: 'post', data: { dept_id } })
}

// 获取所有启用部门（下拉选项）
export function apiGetAllDepts(exclude_dept_id = 0) {
  return request<DeptOptionItem[]>({ url: '/admin/option/depts', method: 'post', data: { exclude_dept_id } })
}

// 新增部门
export function apiAddDept(data: AddDeptRequest) {
  return request<{ status: string }>({ url: '/admin/dept/add', method: 'post', data })
}

// 更新部门
export function apiUpdateDept(data: UpdateDeptRequest) {
  return request<{ status: string }>({ url: '/admin/dept/update', method: 'post', data })
}

// 更新部门状态
export function apiUpdateDeptStatus(dept_id: number, status: number) {
  return request<{ status: string }>({
    url: '/admin/dept/update-status',
    method: 'post',
    data: { dept_id, status },
  })
}

// 删除部门（软删）
export function apiDeleteDept(dept_id: number) {
  return request<{ status: string }>({ url: '/admin/dept/delete', method: 'post', data: { dept_id } })
}

// 获取部门员工选项（最小字段：user_id / username / realname）。
// 选项接口不做接口权限校验，后端刻意不返回手机号等敏感字段。
export function apiGetDeptUserOptions(dept_id: number, page_no = 1, page_size = 100) {
  return request<UserOptionListResponse>({
    url: '/admin/option/dept-users',
    method: 'post',
    data: { dept_id, page_no, page_size },
  })
}

// 批量更新部门员工（穿梭框保存）
export function apiBatchUpdateDeptUser(data: BatchUpdateDeptUserRequest) {
  return request<{ status: string }>({ url: '/admin/dept/batch-update-user', method: 'post', data })
}
