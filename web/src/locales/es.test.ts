/**
 * Smoke coverage for the Spanish (es) catalog. Asserts that the factory
 * wires `es` in correctly and that the key paths that matter for the
 * Login pilot resolve to Spanish strings — not raw keys, not English
 * fallbacks. The key-parity CI gate (scripts/i18n-check-parity.mjs) is
 * the broader coverage for drift; these tests are the pilot's runtime
 * sanity check.
 */
import { describe, it, expect, beforeEach } from 'vitest'
import { i18n, supportedLocales, resolveLocale } from './index'

describe('Spanish (es) catalog', () => {
  beforeEach(() => {
    i18n.global.locale.value = 'es'
  })

  it('is listed in the supported locales', () => {
    expect(supportedLocales).toContain('es')
  })

  it('resolves Spanish auth strings', () => {
    expect(i18n.global.t('login.submit.idle')).toBe('Iniciar sesión')
    expect(i18n.global.t('login.title')).toBe('Plataforma Kootenai')
    expect(i18n.global.t('login.errors.fallback')).toBe('Error al iniciar sesión')
  })

  it('resolves Spanish difficulty labels per the glossary', () => {
    expect(i18n.global.t('difficulty.beginner')).toBe('Principiante')
    expect(i18n.global.t('difficulty.intermediate')).toBe('Intermedio')
    expect(i18n.global.t('difficulty.advanced')).toBe('Avanzado')
    expect(i18n.global.t('difficulty.expert')).toBe('Experto')
  })

  it('keeps product names untranslated (per glossary policy)', () => {
    // Proxmox, CloudStack are brand names and stay English in every locale.
    expect(i18n.global.t('platform.proxmox')).toBe('Proxmox')
    expect(i18n.global.t('platform.cloudstack')).toBe('CloudStack')
  })

  it('interpolates provider name in OAuth CTA', () => {
    expect(i18n.global.t('login.oauth.continueWith', { provider: 'GitHub' })).toBe(
      'Continuar con GitHub',
    )
  })

  it('maps regional Spanish BCP-47 tags to `es`', () => {
    expect(resolveLocale('es-MX')).toBe('es')
    expect(resolveLocale('es-AR')).toBe('es')
    expect(resolveLocale('es-ES')).toBe('es')
  })

  it('resolves the labs catalog header strings', () => {
    expect(i18n.global.t('labs.title')).toBe('Catálogo de laboratorios')
    expect(i18n.global.t('labs.filters.difficulty')).toBe('Dificultad')
    expect(i18n.global.t('labs.card.launchAction')).toBe('Lanzar')
  })

  it('resolves the pods catalog header strings', () => {
    expect(i18n.global.t('pods.title')).toBe('Mis Pods')
    expect(i18n.global.t('pods.actionLabels.destroy')).toBe('Destruir')
  })

  it('resolves the dashboard widget labels', () => {
    expect(i18n.global.t('dashboard.widgets.stats')).toBe('Resumen de estadísticas')
    expect(i18n.global.t('dashboard.stats.totalPoints')).toBe('Puntos totales')
    expect(i18n.global.t('dashboard.quickActions.browseLabs')).toBe('Explorar laboratorios')
  })

  it('interpolates counts in the result counter (labs) and VM counter (pods)', () => {
    expect(i18n.global.t('labs.resultCount', { shown: 5, total: 12 })).toBe('5 de 12 laboratorios')
    expect(i18n.global.t('pods.card.vmsCount', { count: 3 })).toBe('3 VMs')
  })

  it('resolves Priority-2 copy (pod detail, sessions list, session view)', () => {
    expect(i18n.global.t('podDetail.backLabel')).toBe('Volver a Pods')
    expect(i18n.global.t('podDetail.actions.startSession')).toBe('Iniciar sesión')
    expect(i18n.global.t('sessionsList.title')).toBe('Sesiones')
    expect(i18n.global.t('sessionsList.filters.status.active')).toBe('Solo activas')
    expect(i18n.global.t('session.workspace.title')).toBe('Espacio de trabajo del laboratorio')
    expect(i18n.global.t('session.panels.assessment')).toBe('Evaluación')
  })

  it('interpolates session-view placeholders', () => {
    expect(i18n.global.t('session.console.connectingTo', { name: 'vm-1' })).toBe(
      'Conectando a vm-1...',
    )
    expect(i18n.global.t('session.workspace.nudge.heading', { name: 'Config firewall' })).toBe(
      '¿Tienes problemas con "Config firewall"?',
    )
    expect(
      i18n.global.t('sessionsList.fields.scoreValue', { earned: 80, total: 100, percent: 80 }),
    ).toBe('80 / 100 puntos (80%)')
  })
})
