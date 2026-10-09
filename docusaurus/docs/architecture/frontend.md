# Frontend Architecture

The Kootenai frontend is built with Vue.js 3 and uses **Volt PrimeVue** as its component library. This document describes the frontend architecture and component system.

## Technology Stack

| Technology | Purpose |
|------------|---------|
| Vue.js 3 | Frontend framework (Composition API) |
| Volt PrimeVue | UI component library (Tailwind-native) |
| Tailwind CSS v4 | Utility-first CSS framework |
| Pinia | State management |
| Vue Router | Client-side routing |
| TypeScript | Type safety |
| Vite | Build tool and dev server |

## Volt PrimeVue Component Library

### What is Volt?

Volt is a Tailwind-native component library built on PrimeVue's unstyled core. Unlike traditional PrimeVue which uses its own theming system, Volt components are styled entirely with Tailwind CSS classes.

**Key Benefits:**

1. **Pure Tailwind** - No competing theming systems, just Tailwind classes
2. **Local ownership** - Components are copied into your codebase, allowing full customization
3. **Unstyled core** - Uses PrimeVue's headless/unstyled mode for accessibility and functionality
4. **Dark mode ready** - Built-in dark mode support via Tailwind's `dark:` variants

### Directory Structure

```
web/src/
├── components/
│   └── volt/           # Volt component library (local copy)
│       ├── Button.vue
│       ├── Card.vue
│       ├── Dialog.vue
│       ├── InputText.vue
│       ├── Tag.vue
│       ├── ...
│       └── index.ts
├── layout/             # Application layout components
│   ├── AppLayout.vue
│   ├── AppTopbar.vue
│   ├── AppSidebar.vue
│   ├── AppMenu.vue
│   └── composables/
│       └── useLayout.ts
└── views/              # Page components
```

### Import Pattern

Volt components are imported using the `@volt` path alias:

```typescript
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
```

The alias is configured in `vite.config.ts`:

```typescript
resolve: {
  alias: {
    '@': fileURLToPath(new URL('./src', import.meta.url)),
    '@volt': fileURLToPath(new URL('./src/components/volt', import.meta.url))
  }
}
```

## Component Categories

### Layout Components

| Component | Purpose |
|-----------|---------|
| `AppLayout.vue` | Main application wrapper with sidebar/topbar |
| `AppTopbar.vue` | Top navigation bar with user menu |
| `AppSidebar.vue` | Collapsible sidebar navigation |
| `AppMenu.vue` | Navigation menu items |

### Volt UI Components (Commonly Used)

| Component | Purpose |
|-----------|---------|
| `Button` | Primary action buttons with loading states |
| `Card` | Content containers with header/content/footer slots |
| `Tag` | Status badges and labels |
| `Message` | Alert/notification banners |
| `Dialog` | Modal dialogs |
| `InputText` | Text input fields |
| `Select` | Dropdown select inputs |
| `Checkbox` | Checkbox inputs |
| `RadioButton` | Radio button inputs |
| `ProgressBar` | Progress indicators |
| `ProgressSpinner` | Loading spinners |
| `ConfirmDialog` | Confirmation modal with `useConfirm()` |
| `Toast` | Toast notifications with `useToast()` |
| `Menu` | Dropdown/popup menus |
| `Tabs/TabList/Tab/TabPanels/TabPanel` | Tabbed interfaces |
| `Breadcrumb` | Navigation breadcrumbs |
| `SelectButton` | Segmented button groups |

### Console Components (Custom)

These components handle VNC/SPICE console connections and are **not** part of Volt:

| Component | Purpose |
|-----------|---------|
| `VncConsole.vue` | noVNC-based VNC console |
| `SpiceConsole.vue` | SPICE console wrapper |
| `DirectSpiceConsole.vue` | Direct SPICE connection |
| `VmConsole.vue` | Unified console interface |

## Theming

### Tailwind v4 Configuration

Tailwind v4 uses CSS-based configuration with `@theme` blocks. The theme is defined in `src/style.css`:

```css
@import 'tailwindcss';

@theme {
  /* Primary colors */
  --color-primary-50: oklch(0.97 0.02 250);
  --color-primary-500: oklch(0.55 0.2 250);
  --color-primary-600: oklch(0.48 0.2 250);

  /* Surface colors for cards, backgrounds */
  --color-surface-0: oklch(1 0 0);
  --color-surface-50: oklch(0.985 0 0);
  --color-surface-100: oklch(0.965 0 0);
  /* ... */

  /* Dark mode variants are built-in via dark: prefix */
}
```

### Color Conventions

| Color | Usage |
|-------|-------|
| `primary-*` | Primary brand color, buttons, links |
| `surface-*` | Backgrounds, cards, borders |
| `green-*` / `success` | Success states, running status |
| `yellow-*` / `warn` | Warning states, provisioning status |
| `red-*` / `danger` | Error states, destructive actions |
| `blue-*` / `info` | Informational states |

### Dark Mode

Dark mode is supported via Tailwind's `dark:` prefix. Example:

```html
<div class="bg-surface-0 dark:bg-surface-900 text-surface-900 dark:text-surface-100">
  Content adapts to light/dark mode
</div>
```

## PrimeVue Configuration

PrimeVue is configured in unstyled mode in `main.ts`:

```typescript
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ToastService from 'primevue/toastservice'

app.use(PrimeVue, {
  unstyled: true,  // Critical: enables Volt styling
  ripple: false
})
app.use(ConfirmationService)
app.use(ToastService)
```

## Common Patterns

### Status Tags with Severity

```vue
<Tag :value="status" :severity="getStatusSeverity(status)" />

<script setup lang="ts">
function getStatusSeverity(status: string): "success" | "warn" | "danger" | "secondary" {
  switch (status) {
    case 'running': return 'success'
    case 'provisioning': return 'warn'
    case 'error': return 'danger'
    default: return 'secondary'
  }
}
</script>
```

### Confirmation Dialogs

```vue
<template>
  <ConfirmDialog />
  <Button @click="confirmDelete" label="Delete" severity="danger" />
</template>

<script setup lang="ts">
import { useConfirm } from 'primevue/useconfirm'

const confirm = useConfirm()

function confirmDelete() {
  confirm.require({
    message: 'Are you sure you want to delete?',
    header: 'Confirm Delete',
    icon: 'pi pi-exclamation-triangle',
    acceptClass: 'p-button-danger',
    accept: () => performDelete()
  })
}
</script>
```

### Toast Notifications

```vue
<template>
  <Toast />
</template>

<script setup lang="ts">
import { useToast } from 'primevue/usetoast'

const toast = useToast()

function showSuccess() {
  toast.add({
    severity: 'success',
    summary: 'Success',
    detail: 'Operation completed',
    life: 3000
  })
}
</script>
```

### Card with Slots

```vue
<Card>
  <template #title>Card Title</template>
  <template #subtitle>Optional subtitle</template>
  <template #content>
    Main card content goes here
  </template>
  <template #footer>
    <Button label="Action" />
  </template>
</Card>
```

### Loading States

```vue
<div v-if="loading" class="flex justify-center py-12">
  <ProgressSpinner />
</div>

<Button
  :loading="isSubmitting"
  :disabled="isSubmitting"
  label="Submit"
/>
```

## State Management

### Pinia Stores

| Store | Purpose |
|-------|---------|
| `auth.ts` | Authentication state and user info |
| `pods.ts` | Pod list and real-time updates |
| `session.ts` | Active session state |
| `assessment.ts` | Lab assessment results |

### WebSocket Integration

Real-time updates use the `useWebSocket` composable:

```typescript
import { useWebSocket } from '@/composables/useWebSocket'

const ws = useWebSocket({
  podId: pod.id,
  autoConnect: true,
  autoReconnect: true,
  onPodStatusUpdate: (update) => {
    // Handle real-time pod status changes
  }
})
```

## File Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Views | `*View.vue` | `DashboardView.vue` |
| Components | `PascalCase.vue` | `AchievementCard.vue` |
| Composables | `use*.ts` | `useWebSocket.ts` |
| Stores | `*.ts` (in stores/) | `auth.ts` |
| Types | `*.d.ts` or inline | `spice-html5.d.ts` |

## Build Commands

```bash
# Development server
npm run dev

# Type checking
npm run typecheck

# Build for production
npm run build

# Type check + build
npm run build:check
```

## Migration Notes

### From Custom Tailwind to Volt (December 2024)

The frontend was migrated from custom Tailwind CSS components to Volt PrimeVue. Key changes:

1. **Component imports** changed from HTML elements with Tailwind classes to Volt components
2. **Buttons** now use `<Button>` component with `severity`, `size`, `loading` props
3. **Cards** use `<Card>` with `#title`, `#content`, `#footer` slots
4. **Modals** use `<Dialog>` instead of custom modal implementations
5. **Forms** use `<InputText>`, `<Select>`, `<Checkbox>` instead of native inputs
6. **Alerts** use `<Message>` with `severity` prop
7. **Loading** uses `<ProgressSpinner>` and `<ProgressBar>`

### Console Components Unchanged

The VNC/SPICE console components (`VncConsole.vue`, `SpiceConsole.vue`, etc.) were intentionally left unchanged as they have no Volt equivalent and work correctly with their current implementation.
