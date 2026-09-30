import { createRouter, createWebHashHistory } from 'vue-router'
import Home from '../views/Home.vue'
import Discovery from '../views/Discovery.vue'
import Settings from '../views/Settings.vue'
import Logs from '../views/Logs.vue'
import { isWebMode } from '../api'

const routes = [
  { path: '/', component: Home },
  { path: '/discovery', component: Discovery },
  { path: '/settings', component: Settings },
  { path: '/logs', component: Logs },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach((to) => isWebMode() && to.path === '/settings' ? '/' : true)

export default router
