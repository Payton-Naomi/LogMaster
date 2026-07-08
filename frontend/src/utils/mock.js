// 导出判断是否使用Mock
export const useMock = () => import.meta.env.VITE_USE_MOCK === 'true'

// ---------- 模拟数据存储 ----------
let mockTasks = [
  {
    task_id: '202607081030_user_zhangsan',
    project_name: '行车记录仪V3',
    status: 'running',
    created_at: '2026-07-08 10:30:00',
    version: '1.0.0'
  },
  {
    task_id: '202607081035_user_lisi',
    project_name: '行车记录仪V2',
    status: 'success',
    created_at: '2026-07-08 10:35:00',
    version: '0.9.0'
  }
]

let mockRules = [
  { id: 1, keyword: 'ERROR', level: 'error' },
  { id: 2, keyword: 'OutOfMemory', level: 'error' },
  { id: 3, keyword: 'WARN', level: 'warn' }
]

// ---------- 模拟接口函数 ----------

// 上传日志
export const mockUpload = (file, projectName, version) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      const taskId = `2026${Date.now()}_user_${Math.random().toString(36).slice(2, 8)}`
      mockTasks.unshift({
        task_id: taskId,
        project_name: projectName,
        status: 'running',
        created_at: new Date().toISOString().replace('T', ' ').slice(0, 19),
        version: version || '1.0.0'
      })
      resolve({ task_id: taskId })
    }, 500)
  })
}

// 获取任务列表
export const mockGetTasks = (params = {}) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      let list = [...mockTasks]
      if (params.project_name) {
        list = list.filter(t => t.project_name.includes(params.project_name))
      }
      const total = list.length
      const page = params.page || 1
      const pageSize = params.page_size || 20
      const start = (page - 1) * pageSize
      resolve({
        total,
        list: list.slice(start, start + pageSize)
      })
    }, 300)
  })
}

// 获取任务详情
export const mockGetTaskDetail = (taskId) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      const task = mockTasks.find(t => t.task_id === taskId)
      if (task) resolve(task)
      else reject(new Error('任务不存在'))
    }, 200)
  })
}

// 删除任务
export const mockDeleteTask = (taskId) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      const index = mockTasks.findIndex(t => t.task_id === taskId)
      if (index !== -1) {
        mockTasks.splice(index, 1)
        resolve({})
      } else {
        reject(new Error('任务不存在'))
      }
    }, 200)
  })
}

// 获取规则列表
export const mockGetRules = () => {
  return new Promise((resolve) => {
    setTimeout(() => resolve(mockRules), 200)
  })
}

// 新增规则
export const mockAddRule = (keyword, level) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      const newRule = { id: Date.now(), keyword, level }
      mockRules.push(newRule)
      resolve({ id: newRule.id })
    }, 300)
  })
}

// 删除规则
export const mockDeleteRule = (id) => {
  return new Promise((resolve, reject) => {
    setTimeout(() => {
      const index = mockRules.findIndex(r => r.id === id)
      if (index !== -1) {
        mockRules.splice(index, 1)
        resolve({})
      } else {
        reject(new Error('规则不存在'))
      }
    }, 200)
  })
}

// 仪表板统计数据
export const mockDashboardStats = () => {
  return new Promise((resolve) => {
    setTimeout(() => {
      const trend = []
      const now = new Date()
      for (let i = 6; i >= 0; i--) {
        const d = new Date(now)
        d.setDate(d.getDate() - i)
        trend.push({ time: d.toISOString().slice(0, 10), count: Math.floor(Math.random() * 200) + 50 })
      }
      resolve({
        total_logs: 15230,
        error_count: 123,
        trend
      })
    }, 300)
  })
}

// 任务解析结果
export const mockTaskResults = (taskId) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve([
        { keyword: 'ERROR', count: 15, logs: ['2026-07-08 10:30:01 ERROR disk full', '2026-07-08 10:30:05 ERROR camera init fail'] },
        { keyword: 'OutOfMemory', count: 3, logs: ['2026-07-08 10:31:00 OutOfMemory in module x'] }
      ])
    }, 300)
  })
}

// 模拟WebSocket
export const mockWebSocket = (taskId, onMessage, onError, onClose) => {
  if (!useMock()) return { close: () => {} }
  let count = 0
  const timer = setInterval(() => {
    const type = count % 2 === 0 ? 'log' : 'result'
    if (type === 'log') {
      const levels = ['INFO', 'DEBUG', 'WARN', 'ERROR']
      const level = levels[Math.floor(Math.random() * levels.length)]
      onMessage({ type: 'log', content: `${new Date().toISOString().slice(0,19).replace('T',' ')} ${level} Mock log ${count}` })
    } else {
      onMessage({ type: 'result', data: { keyword: ['ERROR','OutOfMemory','WARN'][Math.floor(Math.random()*3)], count: Math.floor(Math.random()*10)+1 } })
    }
    count++
    if (count >= 10) { clearInterval(timer); onClose && onClose() }
  }, 1500)
  return { close: () => clearInterval(timer) }
}

// 获取COM口（模拟）
export const mockGetComPorts = () => {
  return new Promise((resolve) => {
    setTimeout(() => resolve(['COM1', 'COM3', 'COM5']), 200)
  })
}