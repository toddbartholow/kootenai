import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import DashboardToolbar from './DashboardToolbar.vue'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import { i18n } from '@/locales'

function mountToolbar(userName: string) {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/pathways', component: { template: '<div />' } },
      { path: '/labs', component: { template: '<div />' } },
    ],
  })
  return mount(DashboardToolbar, {
    props: { userName },
    global: { plugins: [router, i18n] },
  })
}

describe('DashboardToolbar', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (typeof localStorage !== 'undefined') localStorage.clear()
    i18n.global.locale.value = 'en'
  })

  it('renders the welcome message with the user name', () => {
    const wrapper = mountToolbar('Alice')
    expect(wrapper.text()).toContain('Welcome back, Alice!')
  })

  it('shows the lock icon when the dashboard is locked', () => {
    const wrapper = mountToolbar('Alice')
    const lockBtn = wrapper.find('[aria-label="Unlock dashboard for editing"]')
    expect(lockBtn.exists()).toBe(true)
  })

  it('toggles the dashboard lock when the lock button is clicked', async () => {
    const layout = useDashboardLayout()
    const wrapper = mountToolbar('Alice')

    expect(layout.locked.value).toBe(true)
    await wrapper.find('[aria-label="Unlock dashboard for editing"]').trigger('click')
    expect(layout.locked.value).toBe(false)

    await wrapper.find('[aria-label="Lock dashboard"]').trigger('click')
    expect(layout.locked.value).toBe(true)
  })

  it('renders navigation links for Pathways and Labs', () => {
    const wrapper = mountToolbar('Alice')
    const links = wrapper.findAll('a').map(a => a.attributes('href'))
    expect(links).toContain('/pathways')
    expect(links).toContain('/labs')
  })
})
