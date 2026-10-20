// 下拉选项接口（/admin/option/*，只做登录校验、不做接口权限校验）。
//
// 底座只保留「用户选项」一个例子；部门相关的两个选项接口
// （/admin/option/depts、/admin/option/dept-users）放在 api/dept.ts 里。
// 新增业务下拉时按同样的形式往这里加，并在 app/routes/routes.go 的
// /admin/option 路由组里登记对应 Handler。
import { request } from './http'
import type { UserOptionListResponse } from '../types'

// 用户选项（给「负责人」「经办人」这类人员下拉用）。
// 返回最小字段（user_id / username / realname）：这个接口只做登录校验，
// 不做接口权限校验，后端刻意不返回手机号、邮箱等敏感字段。
export function apiOptionUsers(params: { page_no: number; page_size: number } = { page_no: 1, page_size: 9999 }) {
  return request<UserOptionListResponse>({
    url: '/admin/option/users',
    method: 'post',
    data: { ...params, username_mobile_email: '', status: 1 },
  })
}
