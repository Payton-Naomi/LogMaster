import { createRouter, createWebHistory } from 'vue-router'

const FEISHU_LOGIN_URL = import.meta.env.VITE_FEISHU_LOGIN_URL
let sessionVerified = false

const routes = [
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
router.beforeEach(async (to, from, next) => {
  if (!to.meta.requiresAuth) {
    next()
    return
  }

  if (sessionVerified) {
    next()
    return
  }

  try {
    const response = await fetch('/api/user/info', { credentials: 'same-origin' })
    if (!response.ok) throw new Error('unauthorized')
    const result = await response.json()
    if (result.code !== 0) throw new Error(result.message)
    sessionVerified = true
    localStorage.setItem('user_info', JSON.stringify(result.data))
    next()
  } catch {
    window.location.replace(FEISHU_LOGIN_URL)
  }
})

export default router
