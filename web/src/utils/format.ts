/**
 * Shared formatting utility functions.
 *
 * These are usable outside of `setup()` (e.g., in stores or utility code)
 * and therefore read from the global vue-i18n instance directly rather
 * than `useI18n()`. Inside components, prefer the `useFormatters()`
 * composable for proper reactivity on locale change.
 */
import { i18n } from '@/locales'

function currentLocale(): string {
  return i18n.global.locale.value
}

function tGlobal(key: string, params?: Record<string, unknown>, count?: number): string {
  if (count !== undefined) {
    return i18n.global.t(key, params ?? {}, count)
  }
  return i18n.global.t(key, params ?? {})
}

/**
 * Format a date string or Date object for display using the current locale.
 */
export function formatDate(date: string | Date | undefined, fallback?: string): string {
  if (!date) return fallback ?? tGlobal('common.notAvailable')
  return new Intl.DateTimeFormat(currentLocale()).format(new Date(date))
}

/**
 * Format a duration in minutes using localized unit labels.
 * Examples (en): 30 -> "30m", 90 -> "1h 30m", 120 -> "2h"
 */
export function formatDuration(minutes: number | undefined): string {
  if (!minutes || minutes <= 0) return tGlobal('common.notAvailable')
  if (minutes >= 60) {
    const hours = Math.floor(minutes / 60)
    const mins = minutes % 60
    if (mins === 0) {
      return tGlobal('format.duration.hours', { count: hours }, hours)
    }
    return tGlobal('format.duration.hoursMinutes', { hours, minutes: mins })
  }
  return tGlobal('format.duration.minutes', { count: minutes }, minutes)
}

/**
 * Calculate and format the duration between two timestamps using the current
 * locale's unit labels. If no end timestamp is provided, uses the current time.
 */
export function formatDurationFromTimestamps(start: string, end?: string): string {
  const startDate = new Date(start)
  const endDate = end ? new Date(end) : new Date()
  const diffMins = Math.floor((endDate.getTime() - startDate.getTime()) / 60000)
  return formatDuration(diffMins)
}
