// 跨域通用类型。业务枚举常量（币种、主体类型等）请放到各自的业务类型文件里，
// 这里只留所有列表页都要用的分页结构。

/** 分页查询请求的公共字段（后端所有 *ListRequest 都带这两个字段） */
export interface PageRequest {
  page_no: number
  page_size: number
}

/** 分页响应的公共字段（后端所有 *ListResponse 都带这三个字段） */
export interface PageResponse<T> {
  page_no: number
  page_size: number
  result_total: number
  list: T[]
}
