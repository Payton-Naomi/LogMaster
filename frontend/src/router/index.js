import { createRouter, createWebHistory } from 'vue-router'
// 路由对应页面，后面加页面直接在这加
const routes = [
    { path: '/login', component: () => import('../pages/Login.vue') },
    {
        path: '/',
        component: () => import('../components/Layout.vue'),
        redirect: '/dashboard',
        children: [
            { path: 'dashboard', component: () => import('../pages/Dashboard.vue') },
            { path: 'devices', component: () => import('../pages/DeviceList.vue') },
            { path: 'logs', component: () => import('../pages/LogSearch.vue') }
    ]
    }
]

const router = createRouter({
    history: createWebHistory(),
    routes
})

export default router