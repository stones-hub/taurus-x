// 应用品牌文案的唯一来源。
//
// 项目名由脚手架写进 frontend/.env 的 VITE_APP_TITLE，这里读出来给登录页和顶栏用，
// 避免把项目名硬编码到 .vue 里（.vue 不经过脚手架的 Go 模板渲染）。
// 要改标题改 .env（或本机的 .env.local）即可，不用动组件。

function envText(value: string | undefined, fallback: string): string {
  const text = (value ?? '').trim()
  return text === '' ? fallback : text
}

/** 应用标题：登录页主标题、顶栏品牌文字 */
export const appTitle = envText(import.meta.env.VITE_APP_TITLE, '管理后台')

/** 应用副标题：登录页主标题下方的小字 */
export const appSubtitle = envText(import.meta.env.VITE_APP_SUBTITLE, '管理后台')

/** 品牌首字：登录页圆形 logo 与顶栏方块里的那个字符 */
export const appInitial = appTitle.slice(0, 1).toUpperCase()
