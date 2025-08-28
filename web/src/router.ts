import { createRouter, createWebHistory } from 'vue-router'

// 如果 router.ts 在 src 根目录，路径用 "./views/xxx.vue"
// 如果你的文件在 src/router/index.ts，请把下面的路径改成 "../views/xxx.vue"
import Home from './views/Home.vue'
import Trend from './views/Trend.vue'
import Report from './views/Report.vue'

const routes = [
  { path: '/', name: 'home', component: Home },
  { path: '/trend', name: 'trend', component: Trend },
  { path: '/report', name: 'report', component: Report },
  { path: '/_ping', name: 'ping', component: { template: '<div style="padding:12px">Router OK</div>' } },
  { path: '/:pathMatch(.*)*', redirect: '/' }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  }
})

export default router
