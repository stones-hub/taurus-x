<script setup lang="ts">
// 详情抽屉「字段网格卡」（复用组件）：双列 label/value 展示，灰 label · 深值，四角圆角。
// 状态值可传 tagType 做彩色 el-tag；普通值传 value；长字段 / 备注传 span: 2 独占整行。
// 布局：显式按行分组（span-2 独占一行、span-1 两两成行），用真实 border 画线：
// 末行无下线、每行最右无右线（四边由容器外框承担）。
// 用法：
//   <DetailCard :fields="[
//     { label: '请求方法', value: row.method },
//     { label: '响应状态', value: row.status_code, tagType: row.status_code < 400 ? 'success' : 'danger' },
//     { label: '请求体', value: row.request_body, span: 2 },
//   ]" />
import { computed } from 'vue'

export interface DetailField {
  label: string
  value?: string | number | null
  /** 彩色状态 tag 的 type（'success' | 'primary' | 'warning' | 'danger' | 'info'） */
  tagType?: string
  /** 占整行（长字段 / 备注） */
  span?: 1 | 2
  /** 自定义内容的具名插槽名（用于非纯文本字段） */
  slot?: string
}

const props = defineProps<{
  fields: DetailField[]
}>()

// 显式分行：span-2 独占一行，span-1 按序两两成行（末位落单则单占一行）
const rows = computed<DetailField[][]>(() => {
  const result: DetailField[][] = []
  let pending: DetailField | null = null
  for (const f of props.fields) {
    if ((f.span ?? 1) === 2) {
      if (pending) {
        result.push([pending])
        pending = null
      }
      result.push([f])
    } else {
      if (pending) {
        result.push([pending, f])
        pending = null
      } else {
        pending = f
      }
    }
  }
  if (pending) result.push([pending])
  return result
})
</script>

<template>
  <div class="detail-card">
    <div v-for="(row, ri) in rows" :key="ri" class="detail-row" :class="{ 'last-row': ri === rows.length - 1 }">
      <div
        v-for="(f, ci) in row"
        :key="ci"
        class="detail-cell"
        :class="{ 'last-col': ci === row.length - 1 }"
      >
        <span class="detail-label">{{ f.label }}</span>
        <slot v-if="f.slot" :name="f.slot" />
        <el-tag v-else-if="f.tagType" size="small" :type="f.tagType" class="detail-tag">
          {{ f.value ?? '-' }}
        </el-tag>
        <span v-else class="detail-value">{{ f.value ?? '-' }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.detail-card {
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 1px 3px rgba(16, 24, 40, 0.05), 0 1px 2px rgba(16, 24, 40, 0.03);
  overflow: hidden;
}
.detail-row {
  display: flex;
  /* 行间分隔线：下边框（末行去掉） */
  border-bottom: 1px solid #eef0f3;
}
.detail-row.last-row {
  border-bottom: none;
}
.detail-cell {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 9px 12px;
}
/* 列间分隔线：左列加右边框（每行右列不加） */
.detail-cell:not(.last-col) {
  border-right: 1px solid #eef0f3;
}
.detail-label {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
}
.detail-value {
  font-size: 13px;
  font-weight: 500;
  color: #303133;
  line-height: 1.5;
  word-break: break-word;
  overflow-wrap: break-word;
  white-space: pre-wrap;
}
.detail-tag {
  align-self: flex-start;
}
</style>
