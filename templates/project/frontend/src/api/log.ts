// 系统日志接口（/admin/log/*）。
import { request } from './http'
import type {
  AccessLogListRequest,
  AccessLogListResponse,
  OperationLogListRequest,
  OperationLogListResponse,
  FrontErrorLogListRequest,
  FrontErrorLogListResponse,
} from '../types'

// 获取访问日志列表
export function apiGetAccessLogList(params: AccessLogListRequest) {
  return request<AccessLogListResponse>({ url: '/admin/log/access/list', method: 'post', data: params })
}

// 获取操作日志列表
export function apiGetOperationLogList(params: OperationLogListRequest) {
  return request<OperationLogListResponse>({ url: '/admin/log/operation/list', method: 'post', data: params })
}

// 获取前端错误日志列表
export function apiGetFrontErrorLogList(params: FrontErrorLogListRequest) {
  return request<FrontErrorLogListResponse>({ url: '/admin/log/front-error/list', method: 'post', data: params })
}
