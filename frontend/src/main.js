import { createApp } from 'vue'
import App from './App.vue'
import router from './router' // 引入路由
import './style.css'

const app = createApp(App)
app.use(router) // 使用路由
app.mount('#app')