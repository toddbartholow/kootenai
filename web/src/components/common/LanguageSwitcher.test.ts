import { describe, it, expect, beforeEach } from 'vitest'
import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import LanguageSwitcher from './LanguageSwitcher.vue'
import { useLocaleStore } from '@/stores/locale'
import { i18n } from '@/locales'

// Stub the PrimeVue Select so tests can interact with a plain <select>.
const stubs = {
  Select: {
    props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
    emits: ['update:modelValue'],
    template:
      '<select class="stub-select" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)">' +
      '<option v-for="o in options" :key="o[optionValue]" :value="o[optionValue]">{{ o[optionLabel] }}</option>' +
      '</select>',
  },
}

function mountSwitcher() {
  return mount(LanguageSwitcher, { global: { stubs } })
}

describe('LanguageSwitcher', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    if (typeof localStorage !== 'undefined') localStorage.clear()
    // Reset the i18n locale so each test starts from the default.
    i18n.global.locale.value = 'en'
  })

  it('lists every supported locale with its endonym', () => {
    const wrapper = mountSwitcher()
    const options = wrapper.findAll('option')
    const labels = options.map((o) => o.text())
    expect(labels).toContain('English')
    expect(labels).toContain('Español')
  })

  it('reflects the current locale from the store', () => {
    const store = useLocaleStore()
    store.setLocale('es')
    const wrapper = mountSwitcher()
    const select = wrapper.find<HTMLSelectElement>('.stub-select')
    expect(select.element.value).toBe('es')
  })

  it('writes the chosen locale back to the store on change', async () => {
    const store = useLocaleStore()
    const wrapper = mountSwitcher()
    const select = wrapper.find<HTMLSelectElement>('.stub-select')

    select.element.value = 'es'
    await select.trigger('change')

    expect(store.locale).toBe('es')
  })

  it('updates the global i18n locale when the store changes (via the store watcher)', async () => {
    const store = useLocaleStore()
    store.setLocale('es')
    // Vue's `watch` (non-sync) fires on the next tick after a mutation;
    // wait for it before asserting the propagated i18n locale.
    await nextTick()
    expect(i18n.global.locale.value).toBe('es')
  })

  it('applies an aria-label for screen readers', () => {
    const wrapper = mountSwitcher()
    // The Select component passes aria-label through attribute fallthrough
    // onto the wrapper root.
    const ariaLabel = wrapper.attributes('aria-label')
    expect(ariaLabel).toBe('Language')
  })
})
