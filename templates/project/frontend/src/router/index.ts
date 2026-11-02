// 路由表（页面路径与 admin_permissions.path 对齐）。
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { hasSessionHint } from '../utils/auth-hint'

// 扩展路由 meta 类型：activeMenu 指明当前页面应聚焦的导航菜单 link（绝对路径，如 /page/department/dept.html）。
// 子页面（新增/编辑/详情/操作页）用它显式归属到所属列表页，替代原先脆弱的「路径后缀归一化」反推。
declare module 'vue-router' {
  interface RouteMeta {
    /** 允许匿名访问（无需登录） */
    public?: boolean
    /** 聚焦导航：所属列表页的菜单 link（绝对路径），用于顶栏/侧栏高亮 */
    activeMenu?: string
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/login/index.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    name: 'layout',
    component: () => import('../views/home/index.vue'),
    children: [
      // 默认欢迎页
      {
        path: '',
        name: 'welcome',
        component: () => import('../views/home/welcome.vue'),
      },
      // 个人信息
      {
        path: 'page/profile/profile.html',
        name: 'profile',
        component: () => import('../views/profile/index.vue'),
      },
      // 权限管理
      {
        path: 'page/account/permission.html',
        name: 'permission',
        component: () => import('../views/permission/index.vue'),
      },
      // 权限新增/编辑
      {
        path: 'page/account/permission-add.html',
        name: 'permission-add',
        component: () => import('../views/permission/edit.vue'),
        meta: { activeMenu: '/page/account/permission.html' },
      },
      // 角色管理
      {
        path: 'page/account/role.html',
        name: 'role',
        component: () => import('../views/role/index.vue'),
      },
      // 角色新增/编辑
      {
        path: 'page/account/role-add.html',
        name: 'role-add',
        component: () => import('../views/role/edit.vue'),
        meta: { activeMenu: '/page/account/role.html' },
      },
      // 用户管理
      {
        path: 'page/account/user.html',
        name: 'user',
        component: () => import('../views/user/index.vue'),
      },
      // 用户新增/编辑
      {
        path: 'page/account/user-add.html',
        name: 'user-add',
        component: () => import('../views/user/edit.vue'),
        meta: { activeMenu: '/page/account/user.html' },
      },
      // 登录日志
      {
        path: 'page/log/login.html',
        name: 'login-log',
        component: () => import('../views/log/login.vue'),
      },
      // 访问日志
      {
        path: 'page/log/access.html',
        name: 'access-log',
        component: () => import('../views/log/access.vue'),
      },
      // 操作日志
      {
        path: 'page/log/operation.html',
        name: 'operation-log',
        component: () => import('../views/log/operation.vue'),
      },
      // 前端错误日志
      {
        path: 'page/log/front-error.html',
        name: 'front-error-log',
        component: () => import('../views/log/front-error.vue'),
      },
      // 部门管理
      {
        path: 'page/department/dept.html',
        name: 'dept',
        component: () => import('../views/dept/index.vue'),
      },
      // 部门新增/编辑
      {
        path: 'page/department/dept-add.html',
        name: 'dept-add',
        component: () => import('../views/dept/edit.vue'),
        meta: { activeMenu: '/page/department/dept.html' },
      },
      // 部门员工管理
      {
        path: 'page/department/dept-user-add.html',
        name: 'dept-user-manage',
        component: () => import('../views/dept/user-manage.vue'),
        meta: { activeMenu: '/page/department/dept.html' },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 登录守卫：只看不敏感的登录提示（CSRF Cookie 是否存在，见 utils/auth-hint.ts），
// 不读令牌。提示过时（会话已被吊销）时进入页面后第一个请求会拿到 401，由 http.ts 统一跳回登录页。
router.beforeEach((to) => {
  if (!to.meta.public && !hasSessionHint()) {
    return { name: 'login' }
  }
  return true
})

export default router
