import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/**
 * Locale-aware formatting helpers. Prefer this composable over inline
 * `toLocaleDateString(...)` / `Intl.*` calls so locale switches propagate.
 */
export function useFormatters() {
  const { locale, t } = useI18n()

  const dateFormatter = computed(() => new Intl.DateTimeFormat(locale.value))
  const dateTimeFormatter = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        dateStyle: 'medium',
        timeStyle: 'short',
      }),
  )
  const longDateFormatter = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      }),
  )
  const numberFormatter = computed(() => new Intl.NumberFormat(locale.value))
  const timeFormatter = computed(
    () => new Intl.DateTimeFormat(locale.value, { timeStyle: 'short' }),
  )
  const weekdayShortFormatter = computed(
    () => new Intl.DateTimeFormat(locale.value, { weekday: 'short' }),
  )
  const weekdayDateFormatter = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        weekday: 'long',
        month: 'long',
        day: 'numeric',
        year: 'numeric',
      }),
  )
  const monthDayFormatter = computed(
    () => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric' }),
  )
  const weekdayMonthDayFormatter = computed(
    () =>
      new Intl.DateTimeFormat(locale.value, {
        weekday: 'short',
        month: 'short',
        day: 'numeric',
      }),
  )

  function formatDate(date: string | Date | undefined, fallback?: string): string {
    if (!date) return fallback ?? t('common.notAvailable')
    return dateFormatter.value.format(new Date(date))
  }

  function formatDateTime(date: string | Date | undefined, fallback?: string): string {
    if (!date) return fallback ?? t('common.notAvailable')
    return dateTimeFormatter.value.format(new Date(date))
  }

  function formatLongDate(date: string | Date | undefined, fallback?: string): string {
    if (!date) return fallback ?? t('common.notAvailable')
    return longDateFormatter.value.format(new Date(date))
  }

  function formatTime(date: string | Date | undefined, fallback?: string): string {
    if (!date) return fallback ?? t('common.notAvailable')
    return timeFormatter.value.format(new Date(date))
  }

  function formatNumber(value: number | undefined): string {
    if (value === undefined || value === null) return t('common.notAvailable')
    return numberFormatter.value.format(value)
  }

  /**
   * Short weekday (e.g., "Mon" in en, "lun" in es). Returns the first two
   * characters so narrow UI columns stay legible.
   */
  function formatWeekdayShort(date: string | Date | undefined): string {
    if (!date) return ''
    return weekdayShortFormatter.value.format(new Date(date)).slice(0, 2)
  }

  /** Long date with weekday, e.g., "Monday, March 10, 2026". */
  function formatWeekdayDate(date: string | Date | undefined, fallback?: string): string {
    if (!date) return fallback ?? t('common.notAvailable')
    return weekdayDateFormatter.value.format(new Date(date))
  }

  /** "Mar 10" / "10 mar". */
  function formatMonthDay(date: string | Date | undefined): string {
    if (!date) return ''
    return monthDayFormatter.value.format(new Date(date))
  }

  /** "Mon, Mar 10" / "lun, 10 mar". */
  function formatWeekdayMonthDay(date: string | Date | undefined): string {
    if (!date) return ''
    return weekdayMonthDayFormatter.value.format(new Date(date))
  }

  /**
   * Format a duration in minutes using localized unit labels.
   * Returns `"N/A"` for missing input, otherwise the hour/minute form.
   */
  function formatDuration(minutes: number | undefined): string {
    if (!minutes || minutes <= 0) return t('common.notAvailable')
    if (minutes >= 60) {
      const hours = Math.floor(minutes / 60)
      const mins = minutes % 60
      if (mins === 0) {
        return t('format.duration.hours', { count: hours }, hours)
      }
      return t('format.duration.hoursMinutes', { hours, minutes: mins })
    }
    return t('format.duration.minutes', { count: minutes }, minutes)
  }

  function formatDurationFromTimestamps(start: string, end?: string): string {
    const startDate = new Date(start)
    const endDate = end ? new Date(end) : new Date()
    const diffMins = Math.floor((endDate.getTime() - startDate.getTime()) / 60000)
    return formatDuration(diffMins)
  }

  return {
    locale,
    formatDate,
    formatDateTime,
    formatLongDate,
    formatTime,
    formatNumber,
    formatWeekdayShort,
    formatWeekdayDate,
    formatMonthDay,
    formatWeekdayMonthDay,
    formatDuration,
    formatDurationFromTimestamps,
  }
}
