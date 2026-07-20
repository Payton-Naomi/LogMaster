import { createRouter, createWebHistory } from 'vue-router'

const FEISHU_LOGIN_URL = import.meta.env.VITE_FEISHU_LOGIN_URL
const isDev = import.meta.env.MODE === 'development'

const routes = [
  // 回调页面
  {
    path: '/callback',
    name: 'Callback',
    component: () => import('@/views/Callback.vue'),
    meta: { requiresAuth: false }
  },
  // 主布局
  {
    path: '/',
    component: () => import('@/views/Layout.vue'),
    meta: { requiresAuth: true },
    children: [
      // 默认重定向到实时日志查看
      { path: '', redirect: '/realtime' },
      
      // 第一个菜单：实时日志查看
      { 
        path: 'realtime', 
        name: 'RealtimeLog', 
        component: () => import('@/views/RealtimeLog.vue'), 
        meta: { title: '实时日志查看' } 
      },
      
      // 仪表板
      { 
        path: 'dashboard', 
        name: 'Dashboard', 
        component: () => import('@/views/Dashboard.vue'), 
        meta: { title: '仪表板' } 
      },

      // 测试场景
      {
        path: 'test-scenarios',
        name: 'TestScenarios',
        component: () => import('@/views/TestScenarios.vue'),
        meta: { title: '测试场景' }
      },
      
      // 日志上传
      { 
        path: 'upload', 
        name: 'Upload', 
        component: () => import('@/views/Upload.vue'), 
        meta: { title: '日志上传' } 
      },

      // 日志记录
      {
        path: 'log-records',
        name: 'LogRecords',
        component: () => import('@/views/LogRecords.vue'),
        meta: { title: '日志记录' }
      },
      
      // 任务列表
      { 
        path: 'tasks', 
        name: 'TaskList', 
        component: () => import('@/views/TaskList.vue'), 
        meta: { title: '任务列表' } 
      },
      
      // 任务详情
      { 
        path: 'task/:taskId', 
        name: 'TaskDetail', 
        component: () => import('@/views/TaskDetail.vue'), 
        meta: { title: '任务详情' } 
      },
      
      // 规则配置
      { 
        path: 'rules', 
        name: 'RuleConfig', 
        component: () => import('@/views/RuleConfig.vue'), 
        meta: { title: '规则配置' } 
      },
      
      // 解析结果
      { 
        path: 'analysis/:taskId', 
        name: 'AnalysisResult', 
        component: () => import('@/views/AnalysisResult.vue'), 
        meta: { title: '解析结果' } 
      }
    ]
  },
  // 404
  { path: '/:pathMatch(.*)*', redirect: '/realtime' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, from, next) => {
  const token = localStorage.getItem('access_token')

  // 开发模式：自动注入模拟 Token
  if (isDev && !token) {
    localStorage.setItem('access_token', 'dev_mock_token_' + Date.now())
    localStorage.setItem('user_info', JSON.stringify({ 
      name: '本地开发', 
      user_id: 'dev_user' 
    }))
    console.log('🔧 [开发模式] 已自动登录，跳过飞书认证')
    next()
    return
  }

  // 生产模式：正常权限校验
  if (to.meta.requiresAuth) {
    if (!token) {
      window.location.href = FEISHU_LOGIN_URL
    } else {
      next()
    }
  } else {
    next()
  }
})

export default router
