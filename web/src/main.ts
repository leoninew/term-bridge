import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from './App.vue'
import { i18n } from './i18n'
import { router } from './router'
import { applyDocumentTheme, resolveInitialTheme, useThemeStore } from './store/theme'
import './styles.css'

applyDocumentTheme(resolveInitialTheme())

const pinia = createPinia()
const app = createApp(App)

app.use(pinia).use(router).use(i18n)
useThemeStore().applyTheme()
app.mount('#app')
