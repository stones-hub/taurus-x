// 列表页通用骨架 composable：收敛「分页列表 + 筛选 + 详情抽屉」三件套的重复样板。
//
// 背景：每个列表页都要手写 loadTableData/search/reset/handlePageChange/
// handleSizeChange + loading/list/pagination 状态 + 详情抽屉的 visible/loading/data，
// 逻辑同构、易错点（result_total 回写、分页哨兵语义、加载态）逐页复刻。
// 本 composable 只抽取「数据加载/分页/抽屉状态」这部分确定重复的骨架；
// 筛选表单字段、表格列、详情渲染仍留在各页面（它们在域间差异大，强行抽象反而扭曲）。
//
// 用法：
//   const { loading, list, pagination, loadTableData, search, reset, openDetail, detail, detailVisible, detailLoading } =
//     useListPage({
//       fetcher: (p) => apiGetDeptList(p),
//       buildParams: (form) => ({ dept_name: form.dept_name, status: form.status }), // 含哨兵映射
//       fetchDetail: (id) => apiGetEditDeptInfo(id as number),
//     })
import { onMounted, reactive, ref, type Ref } from 'vue'

/** 列表接口统一响应形状（ListResponse：list + result_total 总数 + page_no 回写）。 */
export interface ListEnvelope<T> {
  list: T[]
  result_total: number
  page_no: number
  page_size?: number
}

/** fetchDetail 的入参：绝大多数详情接口只需一个主键 id。 */
export interface UseListPageOptions<T, F, P = F> {
  /** 列表请求；入参是 page_no/page_size + buildParams 拼好的完整查询参数。 */
  fetcher: (params: P & { page_no: number; page_size: number }) => Promise<ListEnvelope<T>>
  /** 由筛选 form 拼出查询参数（含分页哨兵语义，如 status 传 -1、外键传 0）。 */
  buildParams: (form: F) => P
  /** 详情加载（可选）；传入后自动提供 openDetail/detail/detailVisible/detailLoading。 */
  fetchDetail?: (id: number | string) => Promise<T>
  /** 从列表行提取详情主键 id（建议显式传；缺省取 row.id）。 */
  pickId?: (row: T) => number | string
  /** 初始 page_size（默认 15）。 */
  defaultPageSize?: number
}

/** 筛选 form 的初始值工厂：resetFilters 时用同一份初始值重置（不含分页字段）。 */
export function useListPage<T, F extends object, P = F>(options: UseListPageOptions<T, F, P>) {
  const { fetcher, buildParams, fetchDetail, pickId, defaultPageSize = 15 } = options

  const loading = ref(false)
  const list = ref<T[]>([]) as Ref<T[]>

  // 分页状态独立于 form：form 里不再塞 page_no/page_size，避免 resetFilters 误回退分页。
  const pagination = reactive({
    page_no: 1,
    page_size: defaultPageSize,
    result_total: 0,
  })

  // 筛选 form（由调用方初始化后传入原始值；这里持有引用用于 reset）
  const form = reactive({}) as F
  // 记录初始值快照，供 resetFilters 精确还原
  let initial: F = {} as F

  /** 供调用方在组件 setup 里初始化 form 与初始快照。 */
  function initForm(source: F) {
    Object.assign(form, source)
    initial = JSON.parse(JSON.stringify(source))
  }

  async function loadTableData() {
    loading.value = true
    try {
      const params = buildParams({ ...form }) as P & { page_no: number; page_size: number }
      params.page_no = pagination.page_no
      params.page_size = pagination.page_size
      const res = await fetcher(params)
      list.value = res.list ?? []
      pagination.result_total = res.result_total
      pagination.page_no = res.page_no
      pagination.page_size = res.page_size ?? pagination.page_size
    } finally {
      loading.value = false
    }
  }

  async function search() {
    pagination.page_no = 1
    await loadTableData()
  }

  async function reset() {
    Object.keys(initial).forEach((k) => {
      ;(form as Record<string, unknown>)[k] = (initial as Record<string, unknown>)[k]
    })
    await search()
  }

  async function handlePageChange(page: number) {
    pagination.page_no = page
    await loadTableData()
  }

  async function handleSizeChange(size: number) {
    pagination.page_size = size
    pagination.page_no = 1
    await loadTableData()
  }

  // ===== 详情抽屉 =====
  const detailVisible = ref(false)
  const detailLoading = ref(false)
  const detail = ref<T | null>(null) as Ref<T | null>

  async function openDetail(row: T) {
    if (!fetchDetail) return
    const id = pickId ? pickId(row) : (row as { id?: number | string }).id
    if (id === undefined || id === null) return
    detailVisible.value = true
    detailLoading.value = true
    try {
      detail.value = await fetchDetail(id)
    } finally {
      detailLoading.value = false
    }
  }

  // 首屏加载。必须接住错误：这里是个浮空的 Promise，失败就是控制台的
  // Uncaught (in promise)，还会被 utils/frontError.ts 当成前端 JS 异常上报。
  // 提示已由 api/http.ts 的拦截器弹过，这里不重复弹；loading 由 loadTableData
  // 自己的 finally 复位。
  onMounted(() => {
    loadTableData().catch(() => {})
  })

  return {
    loading,
    list,
    pagination,
    form,
    initForm,
    loadTableData,
    search,
    reset,
    handlePageChange,
    handleSizeChange,
    detailVisible,
    detailLoading,
    detail,
    openDetail,
  }
}
