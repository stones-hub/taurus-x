// 角色相关类型（app/dto/role_dto.go）。

import type { AdminPermission } from './permission'

/** 单个角色详情（含是否选中）（对应后端 dto.UserRoleDetail） */
export interface UserRoleDetail {
    role: AdminRole
    checked: boolean
}

/** 用户角色/权限详情（对应后端 dto.UserRoleDetailResponse） */
export interface UserRoleDetailResponse {
    user_id: number
    roles: UserRoleDetail[]
    all_permissions: AdminPermission[]
}

/** 角色列表响应（对应后端 dto.RoleListResponse） */
export interface RoleListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: RoleListResponseItem[]
}

/** 角色列表项（对应后端 dto.RoleListResponseItem） */
export interface RoleListResponseItem {
    role_id: number
    role_name: string
    role_code: string
    description: string
    sort_order: number
    status: number
    is_system: number
    update_time: string
    add_time: string
}

/** 角色实体（对应后端 model.AdminRole） */
export interface AdminRole {
    role_id: number
    role_name: string
    role_code: string
    description: string
    sort_order: number
    status: number
    is_system: number
    created_at: string
    updated_at: string
}

/** 新增角色请求（对应后端 dto.AddRoleRequest） */
export interface AddRoleRequest {
    role_name: string
    role_code: string
    description: string
    sort_order: number
    permission_ids: number[]
}

/** 更新角色请求（对应后端 dto.UpdateRoleRequest） */
export interface UpdateRoleRequest {
    role_id: number
    role_name: string
    permission_ids: number[]
}

/** 角色列表查询请求（对应后端 dto.RoleListRequest） */
export interface RoleListRequest {
    page_no: number
    page_size: number
    role_name: string
    status: number
}

/** 仅角色 ID 的请求（对应后端 dto.RoleIDRequest） */
export interface RoleIDRequest {
    role_id: number
}

/** 更新角色状态请求（对应后端 dto.UpdateRoleStatusRequest） */
export interface UpdateRoleStatusRequest {
    role_id: number
    status: number
}

/** 仅用户 ID 的请求（对应后端 dto.UserIDRequest） */
export interface UserIDRequest {
    user_id: number
}

/** 分配用户角色请求（对应后端 dto.AssignUserRolesRequest） */
export interface AssignUserRolesRequest {
    user_id: number
    role_ids: number[]
}

/** 编辑角色信息响应（对应后端 dto.EditRoleInfoResponse） */
export interface EditRoleInfoResponse {
    role_id: number
    role_name: string
    role_code: string
    description: string
    sort_order: number
    status: number
    is_system: number
    add_time: string
    update_time: string
    permissions: EditRolePermissionTree[]
}

/** 编辑角色权限树节点（对应后端 dto.EditRolePermissionTree） */
export interface EditRolePermissionTree {
    permission_id: number
    parent_id: number
    permission_name: string
    permission_code: string
    permission_type: number
    path: string
    icon: string
    sort_order: number
    status: number
    is_system: number
    checked: boolean
    children: EditRolePermissionTree[]
}
