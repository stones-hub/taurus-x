// 登录日志相关类型（app/dto/log_dto.go）。

/** 登录日志列表项（对应后端 dto.LoginLogResponse） */
export interface LoginLogResponse {
    log_id: number
    user_id: number
    username: string
    login_type: string
    login_ip: string
    user_agent: string
    login_status: number
    failure_reason: string
    login_time: string
}

/** 登录日志列表响应（对应后端 dto.LoginLogListResponse） */
export interface LoginLogListResponse {
    page_no: number
    page_size: number
    result_total: number
    list: LoginLogResponse[]
}

/** 登录日志查询请求（对应后端 dto.LoginLogListRequest） */
export interface LoginLogListRequest {
    page_no: number
    page_size: number
    login_status: number
}

/** 访问日志列表项（对应后端 dto.AccessLogResponse） */
export interface AccessLogResponse {
  log_id: number
  user_id: number
  username: string
  method: string
  path: string
  status_code: number
  ip: string
  user_agent: string
  duration_ms: number
  query: string
  request_body: string
  response_body: string
  created_at: string
}

/** 访问日志列表响应 */
export interface AccessLogListResponse {
  page_no: number
  page_size: number
  result_total: number
  list: AccessLogResponse[]
}

/** 访问日志查询请求 */
export interface AccessLogListRequest {
  page_no: number
  page_size: number
  user_id?: number
  method?: string
  path?: string
  status_code?: number
  ip?: string
}

/** 操作日志列表项（对应后端 dto.OperationLogResponse） */
export interface OperationLogResponse {
  log_id: number
  user_id: number
  username: string
  method: string
  path: string
  action: string
  resource: string
  ip: string
  status_code: number
  request_body: string
  response_body: string
  result: string
  error_msg: string
  created_at: string
}

/** 操作日志列表响应 */
export interface OperationLogListResponse {
  page_no: number
  page_size: number
  result_total: number
  list: OperationLogResponse[]
}

/** 操作日志查询请求 */
export interface OperationLogListRequest {
  page_no: number
  page_size: number
  user_id?: number
  method?: string
  path?: string
  action?: string
  resource?: string
}

/** 前端错误日志列表项（对应后端 dto.FrontErrorLogResponse） */
export interface FrontErrorLogResponse {
  log_id: number
  user_id: number
  username: string
  kind: string
  message: string
  source: string
  stack: string
  url: string
  user_agent: string
  ip: string
  created_at: string
}

/** 前端错误日志列表响应 */
export interface FrontErrorLogListResponse {
  page_no: number
  page_size: number
  result_total: number
  list: FrontErrorLogResponse[]
}

/** 前端错误日志查询请求 */
export interface FrontErrorLogListRequest {
  page_no: number
  page_size: number
  user_id?: number
  kind?: string
  url?: string
}
