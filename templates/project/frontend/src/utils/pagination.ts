export async function collectPages<Item>(fetchPage: (page: number, size: number) => Promise<{
  list: Item[]
  result_total: number
}>, keyOf?: (item: Item) => number): Promise<Item[]> {
  const items: Item[] = []
  const keys = new Set<number>()
  let total: number | undefined
  for (let page = 1; ; page++) {
    const response = await fetchPage(page, 200)
    if (!Number.isSafeInteger(response.result_total) || response.result_total < 0 || (total !== undefined && total !== response.result_total)) {
      throw new Error('分页数据发生变化，请重新加载')
    }
    total = response.result_total
    if (keyOf) {
      for (const item of response.list) {
        const key = keyOf(item)
        if (keys.has(key)) throw new Error('分页数据重复，请重新加载')
        keys.add(key)
      }
    }
    items.push(...response.list)
    if (items.length === total) return items
    if (items.length > total) throw new Error('分页数据发生变化，请重新加载')
    if (response.list.length === 0) throw new Error('分页数据发生变化，请重新加载')
  }
}
