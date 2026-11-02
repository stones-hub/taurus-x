// 认证相关类型（app/dto/auth_dto.go）。

/** 登录请求（对应后端 dto.LoginRequest） */
export interface LoginRequest {
    username: string
    password: string
}

/**
 * 登录 / 刷新返回（对应后端 dto.LoginResponse）。
 * 不含令牌：令牌只在 HttpOnly Cookie 里。刷新接口的 user 为 null。
 */
export interface LoginResponse {
    user: import('./user').UserInfo | null
    expires_at: string
}
