// 下拉选项加载 composable：从 /admin/option/* 选项接口拉全量（启用状态）选项。
import { ref } from 'vue'

export interface OptionItem {
  value: number
  label: string
}

/**
 * 从选项接口拉全量选项。
 * @param fetcher 选项接口，() => Promise<{list: T[]}>
 * @param pickValue 从 list item 提取 value（id）
 * @param pickLabel 从 list item 提取 label（名称）
 */
export function useOptions<T>(
  fetcher: () => Promise<{ list: T[] }>,
  pickValue: (item: T) => number,
  pickLabel: (item: T) => string,
) {
  const options = ref<OptionItem[]>([])
  const loading = ref(false)

  async function load() {
    loading.value = true
    try {
      const res = await fetcher()
      options.value = (res.list ?? []).map((it) => ({ value: pickValue(it), label: pickLabel(it) }))
    } finally {
      loading.value = false
    }
  }

  return { options, loading, load }
}
