import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLocaleStore } from './locale'
import { i18n } from '@/locales'
import { api } from '@/api'

describe('useLocaleStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    i18n.global.locale.value = 'en'
  })

  it('defaults to en when browser preference is unsupported', () => {
    const store = useLocaleStore()
    expect(store.locale).toBe('en')
  })

  it('exposes supportedLocales', () => {
    const store = useLocaleStore()
    expect(store.supportedLocales).toContain('en')
  })

  it('setLocale updates store and vue-i18n global locale', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('en')
    expect(store.locale).toBe('en')
    expect(i18n.global.locale.value).toBe('en')
    putSpy.mockRestore()
  })

  it('setting locale updates document.documentElement.lang', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('en')
    expect(document.documentElement.lang).toBe('en')
    putSpy.mockRestore()
  })

  it('setting locale sets document.documentElement.dir to ltr for LTR locales', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('es')
    expect(document.documentElement.dir).toBe('ltr')
    store.setLocale('en')
    expect(document.documentElement.dir).toBe('ltr')
    putSpy.mockRestore()
  })

  it('setLocale persists the preference to /auth/me/preferred-locale by default', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('es')
    expect(putSpy).toHaveBeenCalledWith('/auth/me/preferred-locale', { preferredLocale: 'es' })
    putSpy.mockRestore()
  })

  it('setLocale skips persistence when { persist: false } is passed', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('es', { persist: false })
    expect(putSpy).not.toHaveBeenCalled()
    putSpy.mockRestore()
  })

  it('setLocale does not persist the pseudo locale (en-XA)', () => {
    const store = useLocaleStore()
    const putSpy = vi.spyOn(api, 'put').mockResolvedValue({ data: null } as never)
    store.setLocale('en-XA')
    expect(putSpy).not.toHaveBeenCalled()
    putSpy.mockRestore()
  })
})
