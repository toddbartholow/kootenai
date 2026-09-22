import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import PrimeVue from 'primevue/config'
import ToastService from 'primevue/toastservice'
import ConfirmationService from 'primevue/confirmationservice'
import Tooltip from 'primevue/tooltip'
import App from './App.vue'
import router from './router'
import { i18n } from './locales'
import './style.css'

const app = createApp(App)

const pinia = createPinia()
pinia.use(piniaPluginPersistedstate)
app.use(pinia)
app.use(router)
app.use(i18n)
app.use(PrimeVue, { unstyled: true })
app.use(ToastService)
app.use(ConfirmationService)
app.directive('tooltip', Tooltip)

// Global error handler for uncaught Vue errors
app.config.errorHandler = (err, instance, info) => {
  console.error('[Vue Error]', err, { component: instance?.$options?.name, info })
}

// Global handler for unhandled promise rejections
window.addEventListener('unhandledrejection', (event) => {
  console.error('[Unhandled Rejection]', event.reason)
})

app.mount('#app')
