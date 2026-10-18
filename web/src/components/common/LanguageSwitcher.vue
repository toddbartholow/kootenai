<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@volt/Select.vue'
import { useLocaleStore } from '@/stores/locale'
import { localeLabels, type SupportedLocale } from '@/locales'

const { t } = useI18n()

/**
 * Topbar language switcher.
 *
 * The persisted `locale` Pinia store owns the active locale and drives
 * `i18n.global.locale.value` via a watcher. Switching here just writes to
 * the store; the rest of the app re-renders on the next tick. Persistence
 * to localStorage is handled by the store's persist plugin.
 *
 * Options are rendered with each locale's endonym (English, Español, …)
 * so a user searching for their language recognizes the label regardless
 * of the current UI locale.
 */
const store = useLocaleStore()

interface Option {
  value: SupportedLocale
  label: string
}

const options = computed<Option[]>(() =>
  store.supportedLocales.map((value) => ({
    value,
    label: localeLabels[value] ?? value,
  })),
)

const selected = computed<SupportedLocale>({
  get: () => store.locale,
  set: (value) => store.setLocale(value),
})
</script>

<template>
  <Select
    v-model="selected"
    :options="options"
    optionLabel="label"
    optionValue="value"
    class="w-36"
    :aria-label="t('languageSwitcher.aria')"
    data-pseudo-skip
  />
</template>
