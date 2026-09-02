/**
 * Friendly re-exports of the generated OpenAPI schema.
 *
 * The raw types in `schema.ts` are keyed by their fully-qualified Go
 * import paths (e.g.
 * `github_com_toddbartholow_kootenai_api_internal_models.Pod`), which
 * is unpleasant to use directly. This module provides stable, short
 * aliases that consumers import instead.
 *
 * **Do not hand-edit `schema.ts`** — it is regenerated from
 * `api/docs/swagger.json` via `npm run generate:api-types`. CI also
 * verifies the committed file matches via `npm run check:api-types`.
 */
import type { components } from './schema'

type Schema = components['schemas']

// Core domain types. The long keys are the fully-qualified Go package
// paths that swaggo emits. If the Go side ever refactors its package
// layout, these string literals have to be updated in lockstep — the
// generator doesn't abstract them away.
export type ApiPod = Schema['github_com_toddbartholow_kootenai_api_internal_models.Pod']
export type ApiPodVM = Schema['github_com_toddbartholow_kootenai_api_internal_models.PodVM']
export type ApiPodStatus = Schema['github_com_toddbartholow_kootenai_api_internal_models.PodStatus']
export type ApiPodNetwork =
  Schema['github_com_toddbartholow_kootenai_api_internal_models.PodNetwork']
export type ApiPlatform = Schema['github_com_toddbartholow_kootenai_api_internal_models.Platform']

export type ApiSession = Schema['github_com_toddbartholow_kootenai_api_internal_models.Session']

// A lab as the API returns it. This is a flattened LabTemplateRecord, not
// models.LabTemplate — that type is the YAML document shape (apiVersion, kind,
// metadata, spec) and no endpoint returns it. There used to be an
// ApiLabTemplate alias pointing at it, which compiled only because the Go DTOs
// mis-declared these responses; it was unused.
export type ApiLab = Schema['internal_server_labs.LabSummary']

// Raw export for cases where callers need the full schemas map.
export type { components } from './schema'
