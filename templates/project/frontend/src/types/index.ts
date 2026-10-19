// 类型统一出口：与后端 app/dto + app/model 对齐，按域拆分到独立文件。
// 新增业务域时在这里加一行 `export * from './xxx'`。

/** 统一响应包装（对应框架 httpx.Response） */
export interface ApiResponse<T = unknown> {
    code: number
    message: string
    data: T
}

// 跨域通用类型（分页等）
export * from './common'
// 认证
export * from './auth'
// 用户/个人信息
export * from './user'
// 角色
export * from './role'
// 权限
export * from './permission'
// 日志（登录/访问/操作/前端错误）
export * from './log'
// 部门
export * from './dept'
