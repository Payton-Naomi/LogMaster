import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import * as echarts from 'echarts'
import App from './App.vue'
import router from './router'

const app = createApp(App)
// 全局挂载 echarts，所有页面都能直接用 this.$echarts
app.config.globalProperties.$echarts = echarts

app.use(createPinia())
app.use(router)
app.use(ElementPlus)
app.mount('#app')