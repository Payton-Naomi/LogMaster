# Vue 3 + Vite

This template should help get you started developing with Vue 3 in Vite. The template uses Vue 3 `<script setup>` SFCs, check out the [script setup docs](https://v3.vuejs.org/api/sfc-script-setup.html#sfc-script-setup) to learn more.

Learn more about IDE Support for Vue in the [Vue Docs Scaling up Guide](https://vuejs.org/guide/scaling-up/tooling.html#ide-support).

log-frontend/
├── public/ # 静态资源（图标、 favicon 等，不会被编译）
├── src/
│ ├── api/ # 后端接口请求封装（第1周周五联调用）
│ │ └── index.js # axios 实例，统一配置后端地址
│ ├── assets/ # 图片、样式等静态资源
│ ├── components/ # 公共组件（复用性高的组件放这）
│ │ ├── Layout.vue # 整体布局（侧边栏+头部+内容区）
│ │ └── StatCard.vue # 统计卡片组件（仪表板用）
│ ├── pages/ # 页面组件（每个页面对应一个.vue）
│ │ ├── Login.vue # 登录页（第1周周三做）
│ │ ├── Dashboard.vue # 仪表板页（第1周周五扩展）
│ │ ├── DeviceList.vue # 设备列表页（第1周周四做）
│ │ └── LogSearch.vue # 日志检索页（第2周做）
│ ├── router/ # 路由配置
│ │ └── index.js # 页面路由映射
│ ├── store/ # Pinia 状态管理（后续用户登录态用）
│ │ └── index.js
│ ├── utils/ # 工具函数（时间格式化、请求拦截等）
│ ├── App.vue # 根组件
│ └── main.js # 项目入口文件
├── package.json # 依赖配置
├── vite.config.js # Vite 构建配置
└── .gitignore # Git 忽略文件
