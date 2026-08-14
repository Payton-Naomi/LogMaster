import { createApp } from 'vue'
import App from './RootApp.vue'
import 'ant-design-vue/dist/reset.css'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'
import './interface.css'

// 前端崩溃上报：把 window 未捕获异常和未处理的 Promise 拒绝写入采集端诊断日志，
// 便于程序闪退/白屏时通过 %APPDATA%\LogMaster\logs\collector.log 定位原因。
function reportFrontendError(source, event) {
  try {
    const service = window.go?.main?.Service
    if (!service?.LogFrontendError) return
    const reason = event?.reason
    const message = event?.message || (reason !== undefined && reason !== null ? String(reason) : '') || 'unknown frontend error'
    const stack = (event?.error?.stack || event?.stack || '').slice(0, 8192)
    service.LogFrontendError(String(source), String(message), String(stack))
  } catch (_) { /* 上报失败不影响页面 */ }
}
window.addEventListener('error', (event) => reportFrontendError('window', event))
window.addEventListener('unhandledrejection', (event) => reportFrontendError('promise', event))

createApp(App).mount('#app')
