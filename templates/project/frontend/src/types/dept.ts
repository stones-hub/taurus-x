// 部门相关类型（app/dto/dept_dto.go）。

/** 部门列表项（对应后端 dto.DeptListResponseItem） */
export interface DeptListResponseItem {
    dept_id: number
    dept_name: string
    parent_id: number
    parent_dept_name: string
    dept_code: string
    description: string
    sort_order: number
    dept_source: number
    status: number
    add_time: string
    update_time: string
    user_count: number
}

/** 部门列表响应（对应后端 dto.DeptListResponse） */
export interface DeptListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: DeptListResponseItem[]
}

/** 部门列表查询请求（对应后端 dto.DeptListRequest） */
export interface DeptListRequest {
    page_no: number
    page_size: number
    dept_name: string
    status: number
}

/** 新增部门请求（对应后端 dto.AddDeptRequest） */
export interface AddDeptRequest {
    dept_name: string
    parent_id: number
    dept_code: string
    description: string
    sort_order: number
    dept_source: number
}

/** 更新部门请求（对应后端 dto.UpdateDeptRequest） */
export interface UpdateDeptRequest {
    dept_id: number
    dept_name: string
    parent_id: number
    description: string
    sort_order: number
}

/** 编辑部门信息响应（对应后端 dto.EditDeptInfoResponse） */
export interface EditDeptInfoResponse {
    dept_id: number
    dept_name: string
    parent_id: number
    dept_code: string
    description: string
    sort_order: number
    dept_source: number
    status: number
}

/** 部门选项（对应后端 dto.DeptOptionItem） */
export interface DeptOptionItem {
    dept_id: number
    dept_name: string
    parent_id: number
}

/** 部门员工项（对应后端 dto.DeptUserResponse） */
export interface DeptUserResponse {
    id: number
    user_id: number
    username: string
    realname: string
    mobile: string
    is_primary: number
    is_manager: number
    status: number
    add_time: string
}

/** 部门员工列表响应（对应后端 dto.DeptUserListResponse） */
export interface DeptUserListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: DeptUserResponse[]
}

/** 批量更新部门员工请求（对应后端 dto.BatchUpdateDeptUserRequest） */
export interface BatchUpdateDeptUserRequest {
    dept_id: number
    user_ids: number[]
}
