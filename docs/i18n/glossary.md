# Kootenai Translation Glossary

Authoritative term list for translators working on Kootenai locale catalogs. The policy is set in [ADR-0003](../architecture/adr/0003-i18n-glossary-and-rollout.md); this document is the operational reference.

**How to use this document:** before translating a new string, look for each significant noun/verb here. If it's listed, follow the stated translation (or "do not translate"). If it isn't listed, propose a rendering in the PR description so the reviewer can add it here.

**Symbol key:** ✅ translate • 🔒 do not translate (use English verbatim) • ❓ decide case-by-case with the reviewer

---

## Core product nouns

| English | Policy | Notes |
| --- | --- | --- |
| Lab | ✅ | Pedagogical unit. `es`: *Laboratorio*. `pt-BR`: *Laboratório*. |
| Lab Template | ✅ | The YAML-defined blueprint. |
| Pod | 🔒 | Virtualization term of art. In Romance languages it collides with "vainas/vagens" (legumes); in Japanese it gets transliterated as ポッド which adds no clarity. Keep as "Pod". |
| Session | ✅ | Time-bound reservation. |
| Snapshot | 🔒 | Industry-standard term. Every vendor's UI uses it untranslated. |
| Pathway | ✅ | Learning track. `es`: *Ruta de aprendizaje*. `fr`: *Parcours*. |
| Module | ✅ | Subdivision of a pathway. |
| Enrollment | ✅ | `es`: *Inscripción*. |
| Objective | ✅ | Pedagogical use. `es`: *Objetivo*. |
| Achievement | ✅ | `es`: *Logro*. |
| Leaderboard | ✅ | `es`: *Tabla de clasificación*. |
| Certificate | ✅ | `es`: *Certificado*. |

## Infrastructure nouns

| English | Policy | Notes |
| --- | --- | --- |
| VM / Virtual Machine | 🔒 | Universal abbreviation in every locale we target. |
| VLAN | 🔒 | Networking term of art. |
| Subnet | 🔒 | |
| Network Segment | ❓ | `es` can use *Segmento de red*. Check context. |
| Proxmox | 🔒 | Product name. |
| CloudStack | 🔒 | Product name. |
| Canvas | 🔒 | Product name (LMS). |
| OSSEC | 🔒 | Product name. |
| Wazuh | 🔒 | Product name. |

## Roles

| English | Policy | Notes |
| --- | --- | --- |
| Owner | ✅ | Highest permission tier. `es`: *Propietario*. |
| Admin | ✅ | `es`: *Administrador*. |
| Instructor | ✅ | `es`: *Instructor* (same spelling). `fr`: *Instructeur*. |
| Member | ✅ | `es`: *Miembro*. |
| Student | ✅ | `es`: *Estudiante*. |

## Status and difficulty

| English | Policy | Notes |
| --- | --- | --- |
| Beginner | ✅ | `es`: *Principiante*. |
| Intermediate | ✅ | `es`: *Intermedio*. |
| Advanced | ✅ | `es`: *Avanzado*. |
| Expert | ✅ | `es`: *Experto*. |
| Active | ✅ | |
| Inactive | ✅ | |
| Draft | ✅ | `es`: *Borrador*. |
| Published | ✅ | `es`: *Publicado*. |
| Pending | ✅ | `es`: *Pendiente*. |
| Failed | ✅ | `es`: *Fallido*. |
| Completed | ✅ | `es`: *Completado*. |

## Common verbs (user actions)

Translators should pick a verb form appropriate to button context (usually infinitive or imperative, not gerund).

| English | Policy | Notes |
| --- | --- | --- |
| Create | ✅ | `es`: *Crear*. |
| Edit | ✅ | `es`: *Editar*. |
| Delete | ✅ | `es`: *Eliminar*. |
| Save | ✅ | `es`: *Guardar*. |
| Cancel | ✅ | `es`: *Cancelar*. |
| Start a Lab | ✅ | `es`: *Iniciar un laboratorio*. |
| Browse | ✅ | `es`: *Explorar*. |
| Enroll | ✅ | `es`: *Inscribirse*. |
| Reset to Snapshot | ❓ | "Snapshot" stays English; the verb translates. `es`: *Restablecer al snapshot*. |
| Provision | ✅ | `es`: *Aprovisionar*. |

## Product branding

Never translate:
- "Kootenai" (the product name)
- `labctl` (CLI tool)
- `mage`, `ansible-playbook`, `docker compose` (command names)

## Format and measurement

| English | Policy | Notes |
| --- | --- | --- |
| Duration labels (`30m`, `2h`, `1h 30m`) | 🔒 | Keep Latin-script abbreviations even in CJK locales; they fit UI constraints better. |
| "N/A" | ✅ | `es`: *N/D*. `fr`: *N/D*. |
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
