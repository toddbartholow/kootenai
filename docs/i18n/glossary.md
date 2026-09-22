# Kootenai Translation Glossary

Authoritative term list for translators working on Kootenai locale catalogs. The policy is set in [ADR-0003](../architecture/adr/0003-i18n-glossary-and-rollout.md); this document is the operational reference.

**How to use this document:** before translating a new string, look for each significant noun/verb here. If it's listed, follow the stated translation (or "do not translate"). If it isn't listed, propose a rendering in the PR description so the reviewer can add it here.

**Policy column:** *Translate* — render in the target language • *Keep English* — use the English term verbatim • *Decide* — case-by-case with the reviewer

---

## Core product nouns

| English | Policy | Notes |
| --- | --- | --- |
| Lab | Translate | Pedagogical unit. `es`: *Laboratorio*. `pt-BR`: *Laboratório*. |
| Lab Template | Translate | The YAML-defined blueprint. |
| Pod | Keep English | Virtualization term of art. In Romance languages it collides with "vainas/vagens" (legumes); in Japanese it gets transliterated as ポッド which adds no clarity. Keep as "Pod". |
| Session | Translate | Time-bound reservation. |
| Snapshot | Keep English | Industry-standard term. Every vendor's UI uses it untranslated. |
| Pathway | Translate | Learning track. `es`: *Ruta de aprendizaje*. `fr`: *Parcours*. |
| Module | Translate | Subdivision of a pathway. |
| Enrollment | Translate | `es`: *Inscripción*. |
| Objective | Translate | Pedagogical use. `es`: *Objetivo*. |
| Achievement | Translate | `es`: *Logro*. |
| Leaderboard | Translate | `es`: *Tabla de clasificación*. |
| Certificate | Translate | `es`: *Certificado*. |

## Infrastructure nouns

| English | Policy | Notes |
| --- | --- | --- |
| VM / Virtual Machine | Keep English | Universal abbreviation in every locale we target. |
| VLAN | Keep English | Networking term of art. |
| Subnet | Keep English | |
| Network Segment | Decide | `es` can use *Segmento de red*. Check context. |
| Proxmox | Keep English | Product name. |
| CloudStack | Keep English | Product name. |
| Canvas | Keep English | Product name (LMS). |
| OSSEC | Keep English | Product name. |
| Wazuh | Keep English | Product name. |

## Roles

| English | Policy | Notes |
| --- | --- | --- |
| Owner | Translate | Highest permission tier. `es`: *Propietario*. |
| Admin | Translate | `es`: *Administrador*. |
| Instructor | Translate | `es`: *Instructor* (same spelling). `fr`: *Instructeur*. |
| Member | Translate | `es`: *Miembro*. |
| Student | Translate | `es`: *Estudiante*. |

## Status and difficulty

| English | Policy | Notes |
| --- | --- | --- |
| Beginner | Translate | `es`: *Principiante*. |
| Intermediate | Translate | `es`: *Intermedio*. |
| Advanced | Translate | `es`: *Avanzado*. |
| Expert | Translate | `es`: *Experto*. |
| Active | Translate | |
| Inactive | Translate | |
| Draft | Translate | `es`: *Borrador*. |
| Published | Translate | `es`: *Publicado*. |
| Pending | Translate | `es`: *Pendiente*. |
| Failed | Translate | `es`: *Fallido*. |
| Completed | Translate | `es`: *Completado*. |

## Common verbs (user actions)

Translators should pick a verb form appropriate to button context (usually infinitive or imperative, not gerund).

| English | Policy | Notes |
| --- | --- | --- |
| Create | Translate | `es`: *Crear*. |
| Edit | Translate | `es`: *Editar*. |
| Delete | Translate | `es`: *Eliminar*. |
| Save | Translate | `es`: *Guardar*. |
| Cancel | Translate | `es`: *Cancelar*. |
| Start a Lab | Translate | `es`: *Iniciar un laboratorio*. |
| Browse | Translate | `es`: *Explorar*. |
| Enroll | Translate | `es`: *Inscribirse*. |
| Reset to Snapshot | Decide | "Snapshot" stays English; the verb translates. `es`: *Restablecer al snapshot*. |
| Provision | Translate | `es`: *Aprovisionar*. |

## Product branding

Never translate:
- "Kootenai" (the product name)
- `labctl` (CLI tool)
- `mage`, `ansible-playbook`, `docker compose` (command names)

## Format and measurement

| English | Policy | Notes |
| --- | --- | --- |
| Duration labels (`30m`, `2h`, `1h 30m`) | Keep English | Keep Latin-script abbreviations even in CJK locales; they fit UI constraints better. |
| "N/A" | Translate | `es`: *N/D*. `fr`: *N/D*. |
| Dates | n/a | Rendered via `Intl.DateTimeFormat` — no translation needed. |
| Numbers | n/a | Rendered via `Intl.NumberFormat`. |

## Things that are NOT copy

These patterns look like candidates for translation but must stay English:

- HTTP methods (`GET`, `POST`, ...)
- Response codes (`404`, `401`, ...)
- JSON/YAML keys in example code blocks
- Keyboard shortcut labels (`Ctrl+K`, `Esc`, `Enter`)
- Programming language keywords in code samples

## Pluralization

Vue-i18n pluralization uses `|` to separate branches: `"no items | one item | {count} items"`. Different locales need different numbers of branches:

- English, Spanish, French: 3 branches (zero, one, other)
- Russian, Ukrainian, Polish: 4 branches (one, few, many, other)
- Japanese, Chinese, Vietnamese: 1 branch (no grammatical plural)

When adding a plural key in English, assume the target locales may need more branches. Document the intent (`"{count} pods are running"`) in the catalog file as a comment above the key if the translator might lose the context.

## Adding a new term

When you add a new user-facing string that contains a domain-specific noun or verb not listed above:

1. Add the term to the appropriate section of this glossary with a proposed policy.
2. Link the glossary diff from your PR so translators can review.
3. If the term is new to an in-progress translation, ping that locale's reviewer.

Never translate a new domain term based on a dictionary; always consult an SME or existing Kootenai documentation first.
