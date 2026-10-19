// 认证相关类型（app/dto/auth_dto.go）。

/** 登录请求（对应后端 dto.LoginRequest） */
export interface LoginRequest {
    username: string
    password: string
}

/** 注册请求（对应后端 dto.RegisterRequest） */
export interface RegisterRequest {
    username: string
    password: string
}

/** 登录返回（对应后端 dto.LoginResponse） */
export interface LoginResponse {
    token: string
    user: import('./user').UserInfo | null
    expires_at: string
}
