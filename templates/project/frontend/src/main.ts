import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import 'element-plus/dist/index.css'
import './styles/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import App from './App.vue'
import router from './router'
import { permission } from './directives/permission'
import { setupFrontErrorReporting } from './utils/frontError'
import { purgeLegacyToken } from './utils/auth-hint'

// 旧版本把令牌存在 localStorage：现在只用 HttpOnly Cookie，启动时把残留的旧令牌删掉
purgeLegacyToken()

// 安装前端全局错误捕获上报
setupFrontErrorReporting()

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// 注册按钮权限指令
app.directive('permission', permission)

// 全局注册 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.mount('#app')
