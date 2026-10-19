// 权限相关类型（app/dto/permission_dto.go 与 model.AdminPermissions）。

/** 权限类型枚举（对应后端 constants.PermissionType） */
export const PermissionType = {
    Menu: 1,
    Button: 2,
    API: 3,
} as const
export type PermissionType = (typeof PermissionType)[keyof typeof PermissionType]

/** 查询所有状态/类型的哨兵值（-1 = 不过滤）。
 * 用于「启用/禁用」status 字段（取值 0/1，后端 `status >= 0` 判断，0 会被当真实值过滤，须传 -1）。
 * 枚举状态/类型字段（从 1 开始，后端 `> 0` 判断）用 0 即可表示不过滤（见项目规范第 22a 条）。 */
export const QUERY_ALL = -1

/** 前端菜单节点（对应后端 dto.MenuNode） */
export interface MenuNode {
    name: string
    code: string
    sort: number
    mode: string[]
    icon?: string
    link?: string
    children: MenuNode[]
}

/** 用户菜单 + 按钮权限（对应后端 dto.UserMenuButtonResponse） */
export interface UserMenuButtonResponse {
    menus: MenuNode[]
    buttons: string[]
}

/** 权限实体（对应后端 model.AdminPermissions） */
export interface AdminPermission {
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
    created_at: string
    updated_at: string
}

/** 权限列表响应（对应后端 dto.PermissionListResponse） */
export interface PermissionListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: PermissionListResponseItem[]
}

/** 权限列表项（对应后端 dto.PermissionListResponseItem） */
export interface PermissionListResponseItem {
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
    add_time: string
    update_time: string
}

/** 权限树节点（对应后端 dto.PermissionTreeNode） */
export interface PermissionTreeNode {
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
    children: PermissionTreeNode[]
}

/** 编辑权限信息响应（对应后端 dto.EditPermissionInfoResponse） */
export interface EditPermissionInfoResponse {
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
    add_time: string
    update_time: string
    permission_tree: PermissionTreeNode[]
}

/** 权限列表查询请求（对应后端 dto.PermissionListRequest） */
export interface PermissionListRequest {
    page_no: number
    page_size: number
    permission_name: string
    status: number
    permission_type: number
}

/** 权限树查询请求（对应后端 dto.PermissionTreeRequest） */
export interface PermissionTreeRequest {
    parent_id: number
    permission_type: number
}

/** 新增权限请求（对应后端 dto.AddPermissionRequest） */
export interface AddPermissionRequest {
    parent_id: number
    permission_name: string
    permission_code: string
    permission_type: number
    path: string
    icon: string
    sort_order: number
    status: number
    is_system: number
}

/** 更新权限请求（对应后端 dto.UpdatePermissionRequest） */
export interface UpdatePermissionRequest {
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
}

/** 仅权限 ID 的请求（对应后端 dto.PermissionIDRequest） */
export interface PermissionIDRequest {
    permission_id: number
}

/** 更新权限状态请求（对应后端 dto.UpdatePermissionStatusRequest） */
export interface UpdatePermissionStatusRequest {
    permission_id: number
    status: number
}

/** 更新是否系统权限请求（对应后端 dto.UpdatePermissionIsSystemRequest） */
export interface UpdatePermissionIsSystemRequest {
    permission_id: number
    is_system: number
}
