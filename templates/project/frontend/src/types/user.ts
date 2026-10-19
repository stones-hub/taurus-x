// 用户/个人信息相关类型（app/dto/user_dto.go）。

/** 用户信息（对应后端 dto.UserInfo，完整字段对接 admin_users 表） */
export interface UserInfo {
    user_id: number
    username: string
    realname: string
    nickname: string
    avatar: string
    mobile: string | null
    email: string | null
    gender: number
    birthday: string
    position: string
    status: number
    user_source: number
    last_login_time: string
    last_login_ip: string
    created_at: string
    updated_at: string
}

/** 更新个人信息请求（对应后端 dto.UpdateProfileRequest） */
export interface UpdateProfileRequest {
    realname: string
    nickname: string
    email: string
    gender: number
    birthday: string
}

/** 修改密码请求（对应后端 dto.ChangePasswordRequest） */
export interface ChangePasswordRequest {
    old_password: string
    new_password: string
}

/** 设置/重置密码请求（对应后端 dto.SetPasswordRequest） */
export interface SetPasswordRequest {
    new_password: string
}

/** 绑定手机号请求（对应后端 dto.BindMobileRequest） */
export interface BindMobileRequest {
    mobile: string
    code: string
}

/** 解绑手机号请求（对应后端 dto.UnbindMobileRequest） */
export interface UnbindMobileRequest {
    mobile: string
    code: string
}

/** 发送验证码请求（对应后端 dto.SendCodeRequest） */
export interface SendCodeRequest {
    login_type: string
    login_value: string
}

/** 用户列表响应（对应后端 dto.UserListResponse） */
export interface UserListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: UserInfo[]
}

/** 用户列表查询请求（对应后端 dto.UserListRequest） */
export interface UserListRequest {
    page_no: number
    page_size: number
    username_mobile_email: string
    status: number
}

/** 新增用户请求（对应后端 dto.AddUserRequest） */
export interface AddUserRequest {
    username: string
    password: string
    mobile: string
    realname: string
    nickname: string
    avatar: string
    email: string
    gender: number
    birthday: string
    user_source: number
    role_ids: number[]
}

/** 更新用户请求（对应后端 dto.UpdateUserRequest） */
export interface UpdateUserRequest {
    user_id: number
    realname: string
    nickname: string
    password: string
    gender: number
    birthday: string
    email: string
    role_ids: number[]
}

/** 更新用户状态请求（对应后端 dto.UpdateUserStatusRequest） */
export interface UpdateUserStatusRequest {
    user_id: number
    status: number
}
