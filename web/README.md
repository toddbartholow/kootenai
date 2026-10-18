# Kootenai Frontend

Vue.js 3 frontend for the Kootenai platform, built with **Volt PrimeVue** and **Tailwind CSS v4**.

## Quick Start

```bash
# Install dependencies
npm install

# Start development server
npm run dev

# Type check
npm run typecheck

# Build for production
npm run build

# Type check + build
npm run build:check
```

## Technology Stack

- **Vue.js 3** - Composition API with `<script setup>`
- **Volt PrimeVue** - Tailwind-native UI components
- **Tailwind CSS v4** - Utility-first CSS with CSS-based config
- **Pinia** - State management
- **TypeScript** - Type safety
- **Vite** - Build tool

## Project Structure

```
src/
├── api/              # API client and types
├── components/
│   ├── volt/         # Volt UI component library
│   ├── console/      # VNC/SPICE console components
│   ├── assessment/   # Lab assessment components
│   ├── progress/     # Progress tracking components
│   └── reservation/  # Reservation calendar components
├── composables/      # Vue composables (useWebSocket, etc.)
├── layout/           # App layout (sidebar, topbar, menu)
├── stores/           # Pinia state stores
├── types/            # TypeScript type definitions
└── views/            # Page components
```

## Volt Components

Volt is a Tailwind-native component library built on PrimeVue's unstyled core. Components are imported via the `@volt` alias:

```vue
<script setup lang="ts">
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
</script>

<template>
  <Card>
    <template #title>Pod Status</template>
    <template #content>
      <Tag value="running" severity="success" />
      <Button label="Open Console" icon="pi pi-desktop" />
    </template>
  </Card>
</template>
```

### Commonly Used Components

| Component | Purpose |
|-----------|---------|
| `Button` | Actions with loading states |
| `Card` | Content containers |
| `Tag` | Status badges |
| `Message` | Alerts and notifications |
| `Dialog` | Modal dialogs |
| `InputText` | Text inputs |
| `Select` | Dropdowns |
| `ProgressSpinner` | Loading indicators |
| `ConfirmDialog` | Confirmation modals |
| `Toast` | Toast notifications |

## Theming

Theme tokens are defined in `src/style.css` using Tailwind v4's `@theme` blocks:

```css
@theme {
  --color-primary-500: oklch(0.55 0.2 250);
  --color-surface-0: oklch(1 0 0);
  /* ... */
}
```

Dark mode is automatic via Tailwind's `dark:` prefix.

## Documentation

For detailed architecture documentation, see:
- [Frontend Architecture](../docs/docs/architecture/frontend.md)

## Console Components

VNC and SPICE console support is provided by custom components in `src/components/console/`:
- `VncConsole.vue` - noVNC-based VNC viewer
- `SpiceConsole.vue` - SPICE console wrapper
- `VmConsole.vue` - Unified console interface

These components connect to VMs via the backend API's console ticket system.
