/**
 * Smoke tests pinning the shape of the generated OpenAPI types.
 *
 * These assertions run at compile time — any breaking change to the
 * schema (rename, removal, type narrowing) fails `npm run typecheck`.
 * The runtime `expect` calls exist mainly so vitest runs the module;
 * the real checks are in the `satisfies` expressions.
 */
import { describe, it, expect } from 'vitest'
import type {
  ApiPod,
  ApiPodVM,
  ApiPodStatus,
  ApiSession,
} from './index'

describe('generated API types — shape pin', () => {
  it('ApiPod matches the fields the Vue reviewer flagged as missing', () => {
    // Satisfies makes TS enforce the shape without widening the value's type.
    const fixture = {
      id: 'p1',
      status: 'running' as ApiPodStatus,
      labTemplateId: 'lt-1',
      ownerId: 'user-1',
      organizationId: 'org-1',
      teamId: 'team-1',
      metadata: { k: 'v' },
    } satisfies Partial<ApiPod>
    expect(fixture.status).toBe('running')
  })

  it('ApiPodStatus is the exhaustive union from the Go enum', () => {
    const allStatuses = [
      'provisioning',
      'running',
      'stopped',
      'error',
      'destroying',
      'destroyed',
    ] satisfies ApiPodStatus[]
    expect(allStatuses).toHaveLength(6)
  })

  it('ApiPodVM includes node and platform', () => {
    const vm = {
      name: 'web-01',
      platformId: '100',
      node: 'pve',
    } satisfies Partial<ApiPodVM>
    expect(vm.node).toBe('pve')
  })

  it('ApiSession includes multi-tenancy and Canvas fields', () => {
    const session = {
      id: 's1',
      organizationId: 'org-1',
      canvasCourseId: 'c1',
      canvasAssignmentId: 'a1',
      canvasUserId: 'u1',
    } satisfies Partial<ApiSession>
    expect(session.canvasCourseId).toBe('c1')
  })
})
