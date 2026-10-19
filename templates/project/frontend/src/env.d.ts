/// <reference types="vite/client" />

// 本项目用到的 Vite 环境变量（值都来自 frontend/.env*，构建时内联）。
// 这里声明出来是为了有补全和拼写检查；全部可选，组件侧必须给兜底值。
interface ImportMetaEnv {
  /** 后端 API 基地址；dev 走 vite 代理时为 /api，单域名部署时为空 */
  readonly VITE_API_BASE_URL?: string
  /** 应用标题（脚手架按项目名生成），见 src/utils/brand.ts */
  readonly VITE_APP_TITLE?: string
  /** 应用副标题，见 src/utils/brand.ts */
  readonly VITE_APP_SUBTITLE?: string
}

// 让纯 TS 语言服务（如 GoLand 的 TypeScript service）能解析 .vue 导入。
// Volar/vue-tsc 会提供精确的组件类型，这里仅作为兜底的模块声明。
declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
