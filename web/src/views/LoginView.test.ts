import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createWebHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import LoginView from './LoginView.vue'
import { useAuthStore } from '../stores/auth'

// Mock the auth store
vi.mock('../stores/auth', () => ({
  useAuthStore: vi.fn(() => ({
    login: vi.fn(),
    mustChangePassword: false,
    isAuthenticated: false,
    user: null,
    oauth2Providers: [],
    fetchOAuth2Providers: vi.fn(),
    handleOAuth2Callback: vi.fn().mockResolvedValue(false),
  })),
}))

describe('LoginView', () => {
  let router: ReturnType<typeof createRouter>
  let mockLogin: ReturnType<typeof vi.fn>

  beforeEach(() => {
    setActivePinia(createPinia())

    router = createRouter({
      history: createWebHistory(),
      routes: [
        { path: '/login', name: 'login', component: LoginView },
        { path: '/', name: 'dashboard', component: { template: '<div>Dashboard</div>' } },
        {
          path: '/change-password',
          name: 'change-password',
          component: { template: '<div>Change Password</div>' },
        },
        {
          path: '/forgot-password',
          name: 'forgot-password',
          component: { template: '<div>Forgot Password</div>' },
        },
      ],
    })

    mockLogin = vi.fn()
    vi.mocked(useAuthStore).mockReturnValue({
      login: mockLogin,
      mustChangePassword: false,
      isAuthenticated: false,
      user: null,
      oauth2Providers: [],
      fetchOAuth2Providers: vi.fn(),
      handleOAuth2Callback: vi.fn().mockResolvedValue(false),
    } as any)

    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.resetAllMocks()
  })

  function mountComponent() {
    return mount(LoginView, {
      global: {
        plugins: [router],
        stubs: {
          Card: { template: '<div class="card"><slot name="content" /></div>' },
          InputText: {
            template:
              '<input v-model="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
            props: ['modelValue'],
          },
          Password: {
            template:
              '<input type="password" v-model="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
            props: ['modelValue'],
          },
          Button: {
            template:
              '<button :disabled="disabled" @click="$emit(\'click\')"><slot />{{ label }}</button>',
            props: ['label', 'disabled', 'loading'],
          },
          Message: { template: '<div class="message" role="alert"><slot /></div>' },
        },
      },
    })
  }

  describe('initial render', () => {
    it('should render login form', () => {
      const wrapper = mountComponent()

      expect(wrapper.find('form').exists()).toBe(true)
      expect(wrapper.text()).toContain('Kootenai Platform')
      expect(wrapper.text()).toContain('Sign in to your account')
    })

    it('should have email and password inputs', () => {
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      const passwordInput = wrapper.find('input[type="password"]')

      expect(emailInput.exists()).toBe(true)
      expect(passwordInput.exists()).toBe(true)
    })

    it('should have sign in button', () => {
      const wrapper = mountComponent()

      expect(wrapper.text()).toContain('Sign in')
    })

    it('should have demo account button', () => {
      const wrapper = mountComponent()

      expect(wrapper.text()).toContain('Try Demo Account')
    })

    it('should have forgot password link', () => {
      const wrapper = mountComponent()

      expect(wrapper.text()).toContain('Forgot password')
    })
  })

  describe('form validation', () => {
    it('should show error when email is empty', async () => {
      const wrapper = mountComponent()

      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(wrapper.text()).toContain('Email is required')
      expect(mockLogin).not.toHaveBeenCalled()
    })
  })

  describe('login flow', () => {
    it('should call login with email and password', async () => {
      mockLogin.mockResolvedValue(undefined)
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      const passwordInput = wrapper.find('input[type="password"]')

      await emailInput.setValue('user@example.com')
      await passwordInput.setValue('password123')
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(mockLogin).toHaveBeenCalledWith('user@example.com', 'password123')
    })

    it('should redirect to dashboard on successful login', async () => {
      mockLogin.mockResolvedValue(undefined)
      const pushSpy = vi.spyOn(router, 'push')
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      await emailInput.setValue('user@example.com')
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(pushSpy).toHaveBeenCalledWith('/dashboard')
    })

    it('should redirect to change-password when mustChangePassword is true', async () => {
      mockLogin.mockResolvedValue(undefined)
      vi.mocked(useAuthStore).mockReturnValue({
        login: mockLogin,
        mustChangePassword: true,
        isAuthenticated: true,
        user: null,
        oauth2Providers: [],
        fetchOAuth2Providers: vi.fn(),
        handleOAuth2Callback: vi.fn().mockResolvedValue(false),
      } as any)

      const pushSpy = vi.spyOn(router, 'push')
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      await emailInput.setValue('user@example.com')
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(pushSpy).toHaveBeenCalledWith('/change-password')
    })

    it('should display error message on login failure', async () => {
      mockLogin.mockRejectedValue(new Error('Invalid credentials'))
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      await emailInput.setValue('user@example.com')
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(wrapper.text()).toContain('Invalid credentials')
    })

    it('should display API error message if available', async () => {
      mockLogin.mockRejectedValue({
        response: { data: { error: 'Account locked' } },
      })
      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      await emailInput.setValue('user@example.com')
      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      expect(wrapper.text()).toContain('Account locked')
    })
  })

  describe('demo login', () => {
    it('should login with demo credentials when demo button clicked', async () => {
      mockLogin.mockResolvedValue(undefined)
      const wrapper = mountComponent()

      // Find the demo button and click it
      const buttons = wrapper.findAll('button')
      const demoButton = buttons.find(b => b.text().includes('Try Demo Account'))
      expect(demoButton).toBeDefined()

      await demoButton!.trigger('click')
      await flushPromises()

      expect(mockLogin).toHaveBeenCalledWith('demo@example.com', '')
    })
  })

  describe('loading state', () => {
    it('should disable inputs during login', async () => {
      let resolveLogin: (value?: unknown) => void
      mockLogin.mockImplementation(
        () =>
          new Promise(resolve => {
            resolveLogin = resolve
          }),
      )

      const wrapper = mountComponent()

      const emailInput = wrapper.find('input[id="email"]')
      await emailInput.setValue('user@example.com')

      const submitPromise = wrapper.find('form').trigger('submit.prevent')
      await wrapper.vm.$nextTick()

      // Button should show loading state
      expect(wrapper.text()).toContain('Signing in')

      resolveLogin!()
      await submitPromise
      await flushPromises()
    })
  })

  describe('accessibility', () => {
    it('should have proper aria labels', () => {
      const wrapper = mountComponent()

      const form = wrapper.find('form')
      expect(form.attributes('aria-label')).toBe('Login form')
    })

    it('should have required indicators', () => {
      const wrapper = mountComponent()

      expect(wrapper.text()).toContain('*')
      expect(wrapper.html()).toContain('(required)')
    })

    it('should have error role on error message', async () => {
      const wrapper = mountComponent()

      await wrapper.find('form').trigger('submit.prevent')
      await flushPromises()

      const errorMessage = wrapper.find('[role="alert"]')
      expect(errorMessage.exists()).toBe(true)
    })
  })
})
