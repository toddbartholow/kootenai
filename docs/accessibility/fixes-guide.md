# Accessibility Fixes Implementation Guide

This guide provides concrete code examples for fixing the critical accessibility issues identified in the audit.

---

## 1. Add Utility CSS for Screen Reader-Only Content

**File:** `/Users/todd/Downloads/Claude/kootenai/web/src/assets/main.css`

Add this at the end of your main CSS file:

```css
/* Screen reader only - visually hidden but accessible */
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border-width: 0;
}

/* Show on focus for skip links */
.sr-only.focus\:not-sr-only:focus {
  position: static;
  width: auto;
  height: auto;
  padding: 1rem;
  margin: 0;
  overflow: visible;
  clip: auto;
  white-space: normal;
}

/* Ensure focus is always visible */
*:focus-visible {
  outline: 2px solid var(--primary-color);
  outline-offset: 2px;
}

/* Remove default outline but keep for keyboard users */
*:focus:not(:focus-visible) {
  outline: none;
}
```

---

## 2. Fix PathwayRoadmap.vue - Make Modules Keyboard Accessible

**File:** `/Users/todd/Downloads/Claude/kootenai/web/src/components/pathway/PathwayRoadmap.vue`

### Replace lines 149-177 with:

```vue
<!-- Module card -->
<button
  type="button"
  :id="`module-${module.id}`"
  :class="[
    'mx-auto max-w-md w-full transform transition-all duration-200 text-left',
    isModuleClickable(module.id) ? 'cursor-pointer hover:scale-105' : 'cursor-not-allowed opacity-75'
  ]"
  :tabindex="isModuleClickable(module.id) ? 0 : -1"
  :aria-disabled="!isModuleClickable(module.id)"
  :aria-expanded="expandedModuleId === module.id"
  :aria-controls="`module-${module.id}-labs`"
  :aria-labelledby="`module-${module.id}-title`"
  :aria-describedby="`module-${module.id}-desc`"
  @click="isModuleClickable(module.id) && handleModuleClick(module, $event)"
  @keydown.enter="isModuleClickable(module.id) && handleModuleClick(module, $event)"
  @keydown.space.prevent="isModuleClickable(module.id) && handleModuleClick(module, $event)"
>
  <div
    :class="[
      'relative rounded-xl border-2 p-4 shadow-md transition-all',
      isModuleClickable(module.id) ? 'hover:shadow-lg' : '',
      getModuleColor(index).bg,
      getModuleColor(index).border,
      expandedModuleId === module.id ? 'ring-2 ring-primary-500 ring-offset-2 dark:ring-offset-surface-900' : ''
    ]"
  >
    <!-- Status indicator -->
    <div
      class="absolute -top-3 -right-3"
      role="status"
      :aria-label="`Module status: ${getModuleStatus(module.id).replace('_', ' ')}`"
    >
      <div
        :class="[
          'w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md transition-transform',
          getStatusColor(getModuleStatus(module.id)),
          getModuleStatus(module.id) === 'completed' ? 'scale-110' : ''
        ]"
      >
        <i :class="getStatusIcon(getModuleStatus(module.id))" aria-hidden="true" />
        <span class="sr-only">{{ getModuleStatus(module.id).replace('_', ' ') }}</span>
      </div>
    </div>
```

### Replace lines 179-200 with:

```vue
    <!-- Module header -->
    <div class="flex items-start gap-3 mb-3">
      <div
        :class="[
          'w-10 h-10 rounded-lg flex items-center justify-center text-lg font-bold transition-colors',
          getModuleStatus(module.id) === 'completed'
            ? 'bg-green-100 dark:bg-green-900 text-green-600 dark:text-green-400'
            : getModuleColor(index).bg + ' ' + getModuleColor(index).text
        ]"
        aria-hidden="true"
      >
        <i v-if="getModuleStatus(module.id) === 'completed'" class="pi pi-check" />
        <span v-else>{{ index + 1 }}</span>
      </div>
      <div class="flex-1 min-w-0">
        <h3
          :id="`module-${module.id}-title`"
          :class="['font-semibold text-lg truncate', getModuleColor(index).text]"
        >
          {{ module.name }}
        </h3>
        <p
          :id="`module-${module.id}-desc`"
          class="text-sm text-surface-600 dark:text-surface-400 line-clamp-2"
        >
          {{ module.description }}
        </p>
      </div>
    </div>
```

### Replace lines 202-209 with:

```vue
    <!-- Progress bar (when enrolled and has progress) -->
    <div v-if="moduleProgress?.has(module.id) && getModuleStatus(module.id) !== 'locked'" class="mb-3">
      <div class="flex justify-between text-xs text-surface-600 dark:text-surface-300 mb-1">
        <span>Progress</span>
        <span>{{ Math.round(getModuleProgress(module.id)) }}%</span>
      </div>
      <ProgressBar
        :value="getModuleProgress(module.id)"
        :showValue="false"
        class="h-1.5"
        role="progressbar"
        :aria-valuenow="Math.round(getModuleProgress(module.id))"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="`Module progress: ${Math.round(getModuleProgress(module.id))} percent`"
      />
    </div>
```

### Replace lines 211-236 with:

```vue
    <!-- Labs within module (collapsed by default, expanded on click) -->
    <div
      v-if="module.labs?.length"
      :id="`module-${module.id}-labs`"
      :class="[
        'overflow-hidden transition-all duration-300',
        expandedModuleId === module.id ? 'max-h-96 opacity-100' : 'max-h-0 opacity-0'
      ]"
      role="region"
      :aria-label="`Labs for ${module.name}`"
      :hidden="expandedModuleId !== module.id"
    >
      <ul class="space-y-2 pt-3 border-t border-surface-200 dark:border-surface-600">
        <li
          v-for="lab in module.labs"
          :key="lab.id"
        >
          <button
            type="button"
            class="w-full flex items-center gap-2 p-2 rounded-lg bg-white/50 dark:bg-surface-800/50 hover:bg-white dark:hover:bg-surface-800 transition-colors group"
            :aria-label="`Launch ${lab.labName}, duration ${lab.labDurationMinutes} minutes`"
            @click="handleLabClick(lab, module, $event)"
            @keydown.enter="handleLabClick(lab, module, $event)"
            @keydown.space.prevent="handleLabClick(lab, module, $event)"
          >
            <i class="pi pi-desktop text-surface-500 group-hover:text-primary-500 transition-colors" aria-hidden="true" />
            <span class="flex-1 text-sm text-surface-700 dark:text-surface-300 truncate group-hover:text-surface-900 dark:group-hover:text-surface-100 text-left">
              {{ lab.labName }}
            </span>
            <span class="text-xs text-surface-500" aria-label="Duration">
              {{ lab.labDurationMinutes }}min
            </span>
            <i class="pi pi-chevron-right text-xs text-surface-400 group-hover:text-primary-500 transition-colors" aria-hidden="true" />
          </button>
        </li>
      </ul>
    </div>
```

### Replace lines 238-251 with:

```vue
    <!-- Module stats -->
    <div class="flex items-center justify-between mt-3 pt-3 border-t border-surface-200 dark:border-surface-600 text-sm text-surface-600 dark:text-surface-300">
      <span>
        <i class="pi pi-book mr-1" aria-hidden="true" />
        <span class="sr-only">Number of labs: </span>
        {{ module.labCount || module.labs?.length || 0 }} labs
      </span>
      <span>
        <i class="pi pi-star mr-1" aria-hidden="true" />
        <span class="sr-only">Total points: </span>
        {{ module.totalPoints || 0 }} pts
      </span>
      <span v-if="module.labs?.length" class="sr-only">
        {{ expandedModuleId === module.id ? 'Expanded, press Space to collapse' : 'Collapsed, press Space to expand' }}
      </span>
      <i
        v-if="module.labs?.length"
        :class="['pi text-xs transition-transform', expandedModuleId === module.id ? 'pi-chevron-up' : 'pi-chevron-down']"
        aria-hidden="true"
      />
    </div>
  </div>
</button>
```

---

## 3. Fix PathwayInteractiveView.vue - Add Semantic Structure

**File:** `/Users/todd/Downloads/Claude/kootenai/web/src/views/PathwayInteractiveView.vue`

### Replace lines 294-298 with:

```vue
<div class="space-y-6">
  <!-- Skip link -->
  <a
    href="#main-pathway-content"
    class="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:p-4 focus:bg-primary-500 focus:text-white focus:rounded"
  >
    Skip to pathway content
  </a>

  <!-- Loading state -->
  <div
    v-if="loading"
    class="flex justify-center py-12"
    role="status"
    aria-live="polite"
  >
    <ProgressSpinner aria-hidden="true" />
    <span class="sr-only">Loading pathway details, please wait...</span>
  </div>
```

### Replace lines 300-312 with:

```vue
  <!-- Error message (full page) -->
  <div
    v-else-if="error && !pathway"
    role="alert"
    aria-live="assertive"
  >
    <Message severity="error">
      {{ error }}
      <template #closeicon>
        <Button
          @click="router.push('/pathways')"
          label="Back to Pathways"
          severity="secondary"
          size="small"
          class="ml-4"
        />
      </template>
    </Message>
  </div>
```

### Replace lines 314-322 with:

```vue
  <template v-else-if="pathway">
    <!-- Breadcrumb -->
    <nav aria-label="Breadcrumb">
      <ol class="flex items-center gap-2 text-sm text-surface-500">
        <li>
          <router-link
            to="/pathways"
            class="hover:text-surface-700 dark:hover:text-surface-300"
            aria-label="Return to Learning Pathways"
          >
            Learning Pathways
          </router-link>
        </li>
        <li aria-hidden="true">
          <i class="pi pi-chevron-right text-xs" />
        </li>
        <li aria-current="page">
          <span class="text-surface-700 dark:text-surface-300">{{ pathway.name }}</span>
        </li>
      </ol>
    </nav>

    <!-- Main content area -->
    <main id="main-pathway-content">
```

### Replace lines 324-340 with:

```vue
      <!-- Header Section -->
      <header class="flex flex-col lg:flex-row gap-6">
        <div class="flex-1 space-y-4">
          <!-- Tags and Title -->
          <ul class="flex items-center gap-2 mb-2" aria-label="Pathway attributes">
            <li v-if="pathway.isFeatured">
              <Tag value="Featured" severity="warn" role="status" />
            </li>
            <li>
              <Tag
                :value="getDifficultyLabel(pathway.difficulty)"
                :severity="getDifficultySeverity(pathway.difficulty)"
                role="note"
                :aria-label="`Difficulty level: ${getDifficultyLabel(pathway.difficulty)}`"
              />
            </li>
            <li v-if="isEnrolled">
              <Tag value="Enrolled" severity="info" role="status" />
            </li>
          </ul>
          <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ pathway.name }}</h1>
          <p class="text-lg text-surface-600 dark:text-surface-400">{{ pathway.description }}</p>

          <!-- Tags -->
          <ul v-if="pathway.tags?.length" class="flex flex-wrap gap-2" aria-label="Pathway topics">
            <li v-for="tag in pathway.tags" :key="tag">
              <Tag :value="tag" severity="secondary" />
            </li>
          </ul>
        </div>
```

### Replace lines 342-406 with:

```vue
        <!-- Stats Card -->
        <aside aria-labelledby="stats-heading">
          <Card class="lg:w-80 flex-shrink-0">
            <template #content>
              <div class="space-y-4">
                <h2 id="stats-heading" class="sr-only">Pathway Statistics and Progress</h2>

                <!-- Progress (when enrolled) -->
                <div v-if="isEnrolled" class="space-y-2" role="region" aria-label="Your progress">
                  <div class="flex justify-between text-sm">
                    <span class="text-surface-600 dark:text-surface-400">Overall Progress</span>
                    <span class="font-semibold text-surface-900 dark:text-surface-100">{{ overallProgress }}%</span>
                  </div>
                  <ProgressBar
                    :value="overallProgress"
                    :showValue="false"
                    class="h-2"
                    role="progressbar"
                    :aria-valuenow="overallProgress"
                    aria-valuemin="0"
                    aria-valuemax="100"
                    :aria-label="`Overall pathway progress: ${overallProgress} percent complete`"
                  />
                  <p class="text-xs text-surface-500">
                    {{ completedModules }} of {{ modules.length }} modules completed
                  </p>
                </div>

                <!-- Stats Grid -->
                <dl class="grid grid-cols-2 gap-4">
                  <div class="text-center">
                    <dt class="text-sm text-surface-500">Modules</dt>
                    <dd class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ modules.length }}</dd>
                  </div>
                  <div class="text-center">
                    <dt class="text-sm text-surface-500">Labs</dt>
                    <dd class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ totalLabs }}</dd>
                  </div>
                  <div class="text-center">
                    <dt class="text-sm text-surface-500">Estimated Time</dt>
                    <dd class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ formatDuration(pathway.estimatedHours) }}</dd>
                  </div>
                  <div class="text-center">
                    <dt class="text-sm text-surface-500">Total Points</dt>
                    <dd class="text-2xl font-bold text-amber-600 dark:text-amber-400">{{ totalPoints.toLocaleString() }}</dd>
                  </div>
                </dl>

                <div class="border-t border-surface-200 dark:border-surface-700 pt-4">
                  <Button
                    v-if="!isEnrolled"
                    @click="enroll"
                    :loading="enrolling"
                    :disabled="enrolling"
                    label="Enroll Now"
                    icon="pi pi-plus"
                    :aria-label="`Enroll in ${pathway.name} pathway`"
                    class="w-full"
                    size="large"
                  />
                  <Button
                    v-else-if="nextLab"
                    @click="launchLab(nextLab.lab)"
                    :loading="launchingLab === nextLab.lab.labTemplateId"
                    :disabled="!!launchingLab"
                    label="Continue Learning"
                    icon="pi pi-play"
                    :aria-label="`Continue to ${nextLab.lab.labName}`"
                    class="w-full"
                    size="large"
                  />
                  <div v-else class="text-center" role="status" aria-live="polite">
                    <div class="w-12 h-12 mx-auto mb-2 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center">
                      <i class="pi pi-trophy text-xl text-green-600 dark:text-green-400" aria-hidden="true" />
                    </div>
                    <p class="font-semibold text-green-600 dark:text-green-400">Pathway Complete!</p>
                  </div>
                </div>
              </div>
            </template>
          </Card>
        </aside>
      </header>
```

### Replace lines 408-412 with:

```vue
      <!-- Live region for status announcements -->
      <div
        role="status"
        aria-live="polite"
        aria-atomic="true"
        class="sr-only"
      >
        <span v-if="enrolling">Enrolling in pathway, please wait...</span>
        <span v-else-if="enrollment && !error">Successfully enrolled in pathway!</span>
        <span v-if="launchingLab">Launching lab, please wait...</span>
      </div>

      <!-- Inline Error Message -->
      <div v-if="error && pathway" role="alert" aria-live="assertive">
        <Message severity="error" :closable="true" @close="error = null">
          {{ error }}
        </Message>
      </div>
```

### Replace lines 414-427 with:

```vue
      <!-- Prerequisites -->
      <section v-if="pathway.prerequisites?.length" aria-labelledby="prerequisites-heading">
        <Card>
          <template #title>
            <h2 id="prerequisites-heading" class="flex items-center gap-2">
              <i class="pi pi-exclamation-triangle text-amber-500" aria-hidden="true" />
              Prerequisites
            </h2>
          </template>
          <template #content>
            <ul class="list-disc list-inside space-y-1 text-surface-600 dark:text-surface-400">
              <li v-for="(prereq, index) in pathway.prerequisites" :key="index">{{ prereq }}</li>
            </ul>
          </template>
        </Card>
      </section>
```

### Replace lines 429-457 with:

```vue
      <!-- Next Lab Recommendation (when enrolled) -->
      <section
        v-if="isEnrolled && nextLab"
        aria-labelledby="next-lab-heading"
      >
        <Card class="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20">
          <template #content>
            <article>
              <h2 id="next-lab-heading" class="sr-only">Recommended Next Lab</h2>
              <div class="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
                <div class="flex items-center gap-4">
                  <div class="w-12 h-12 rounded-full bg-blue-100 dark:bg-blue-900 flex items-center justify-center" aria-hidden="true">
                    <i class="pi pi-arrow-right text-xl text-blue-600 dark:text-blue-400" />
                  </div>
                  <div>
                    <p class="text-sm text-surface-500">Next up in {{ nextLab.module.name }}</p>
                    <h3 class="font-semibold text-lg text-surface-900 dark:text-surface-100">{{ nextLab.lab.labName }}</h3>
                    <div class="flex items-center gap-4 text-sm text-surface-500 mt-1">
                      <span>
                        <i class="pi pi-clock mr-1" aria-hidden="true" />
                        <span class="sr-only">Duration: </span>
                        {{ formatLabDuration(nextLab.lab.labDurationMinutes) }}
                      </span>
                      <span class="text-amber-600 dark:text-amber-400">
                        <span class="sr-only">Points: </span>
                        {{ nextLab.lab.labMaxPoints }} pts
                      </span>
                      <span v-if="nextLab.attempts > 0">{{ nextLab.attempts }} attempt{{ nextLab.attempts !== 1 ? 's' : '' }}</span>
                    </div>
                  </div>
                </div>
                <Button
                  @click="launchLab(nextLab.lab)"
                  :loading="launchingLab === nextLab.lab.labTemplateId"
                  :disabled="!!launchingLab"
                  :label="nextLab.attempts > 0 ? 'Try Again' : 'Start Lab'"
                  :icon="nextLab.attempts > 0 ? 'pi pi-replay' : 'pi pi-play'"
                  :aria-label="`${nextLab.attempts > 0 ? 'Retry' : 'Start'} ${nextLab.lab.labName}`"
                  size="large"
                />
              </div>
            </article>
          </template>
        </Card>
      </section>
```

### Replace lines 459-472 with:

```vue
      <!-- Roadmap Visualization -->
      <section aria-labelledby="roadmap-heading">
        <h2 id="roadmap-heading" class="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-4">
          Learning Roadmap
        </h2>
        <Card class="overflow-hidden">
          <template #content>
            <PathwayRoadmap
              :modules="modules"
              :module-progress="moduleProgress"
              :on-module-click="handleModuleClick"
              :on-lab-click="handleLabClick"
            />
          </template>
        </Card>
      </section>
```

### Replace lines 474-505 with:

```vue
      <!-- Legend -->
      <section aria-labelledby="legend-heading">
        <Card>
          <template #title>
            <h2 id="legend-heading">Status Legend</h2>
          </template>
          <template #content>
            <ul class="flex flex-wrap gap-6">
              <li class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-green-600 dark:text-green-400" aria-hidden="true">
                  <i class="pi pi-check-circle" />
                </div>
                <span class="text-sm text-surface-600 dark:text-surface-400">Completed</span>
              </li>
              <li class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-blue-600 dark:text-blue-400" aria-hidden="true">
                  <i class="pi pi-spin pi-spinner" />
                </div>
                <span class="text-sm text-surface-600 dark:text-surface-400">In Progress</span>
              </li>
              <li class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-amber-600 dark:text-amber-400" aria-hidden="true">
                  <i class="pi pi-lock-open" />
                </div>
                <span class="text-sm text-surface-600 dark:text-surface-400">Unlocked</span>
              </li>
              <li class="flex items-center gap-2">
                <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-surface-400 dark:text-surface-500" aria-hidden="true">
                  <i class="pi pi-lock" />
                </div>
                <span class="text-sm text-surface-600 dark:text-surface-400">Locked</span>
              </li>
            </ul>
          </template>
        </Card>
      </section>
```

### Replace lines 507-530 with:

```vue
      <!-- Enroll CTA (when not enrolled) -->
      <section v-if="!isEnrolled" aria-labelledby="enroll-cta-heading">
        <Card class="bg-gradient-to-r from-primary-50 to-blue-50 dark:from-primary-900/20 dark:to-blue-900/20">
          <template #content>
            <div class="flex flex-col md:flex-row items-center justify-between gap-4">
              <div>
                <h2 id="enroll-cta-heading" class="text-xl font-semibold text-surface-900 dark:text-surface-100">
                  Ready to start learning?
                </h2>
                <p class="text-surface-600 dark:text-surface-400">
                  Enroll now to track your progress and earn achievements.
                </p>
              </div>
              <Button
                @click="enroll"
                :loading="enrolling"
                :disabled="enrolling"
                label="Enroll Now"
                icon="pi pi-arrow-right"
                iconPos="right"
                :aria-label="`Enroll in ${pathway.name} pathway`"
                size="large"
              />
            </div>
          </template>
        </Card>
      </section>
    </main>
  </template>
```

### Replace lines 532-623 with:

```vue
  <!-- Lab Details Dialog -->
  <Dialog
    v-model:visible="showLabDialog"
    :header="selectedLab?.labName || 'Lab Details'"
    :modal="true"
    :dismissableMask="true"
    :focusOnShow="true"
    :closeOnEscape="true"
    role="dialog"
    :aria-labelledby="selectedLab ? `lab-dialog-title-${selectedLab.id}` : undefined"
    :aria-describedby="selectedLab ? `lab-dialog-desc-${selectedLab.id}` : undefined"
    class="w-full max-w-lg"
  >
    <div v-if="selectedLab && selectedModule" class="space-y-4">
      <h2 :id="`lab-dialog-title-${selectedLab.id}`" class="sr-only">
        {{ selectedLab.labName }} Details
      </h2>

      <!-- Module context -->
      <div class="text-sm text-surface-500">
        <i class="pi pi-folder mr-1" aria-hidden="true" />
        <span class="sr-only">Part of module: </span>
        {{ selectedModule.name }}
      </div>

      <!-- Lab description -->
      <p
        v-if="selectedLab.labDescription"
        :id="`lab-dialog-desc-${selectedLab.id}`"
        class="text-surface-600 dark:text-surface-400"
      >
        {{ selectedLab.labDescription }}
      </p>

      <!-- Lab stats -->
      <dl class="flex flex-wrap gap-4 py-2">
        <div class="flex items-center gap-2">
          <dt class="sr-only">Duration:</dt>
          <i class="pi pi-clock text-surface-400" aria-hidden="true" />
          <dd class="text-surface-600 dark:text-surface-400">{{ formatLabDuration(selectedLab.labDurationMinutes) }}</dd>
        </div>
        <div class="flex items-center gap-2">
          <dt class="sr-only">Points:</dt>
          <i class="pi pi-star-fill text-amber-500" aria-hidden="true" />
          <dd class="text-surface-600 dark:text-surface-400">{{ selectedLab.labMaxPoints }} points</dd>
        </div>
        <div v-if="selectedLab.labDifficulty">
          <Tag
            :value="getDifficultyLabel(selectedLab.labDifficulty)"
            :severity="getDifficultySeverity(selectedLab.labDifficulty)"
            :aria-label="`Difficulty: ${getDifficultyLabel(selectedLab.labDifficulty)}`"
          />
        </div>
      </dl>

      <!-- Progress (if enrolled and has progress) -->
      <div
        v-if="isEnrolled"
        class="p-4 bg-surface-50 dark:bg-surface-800 rounded-lg"
        role="region"
        aria-label="Your lab progress"
      >
        <template v-if="getLabProgress(selectedLab)">
          <div class="flex items-center justify-between mb-2">
            <span class="text-sm text-surface-600 dark:text-surface-400">Your Progress</span>
            <Tag
              :value="getLabProgress(selectedLab)?.passed ? 'Passed' : 'Not Passed'"
              :severity="getLabProgress(selectedLab)?.passed ? 'success' : 'secondary'"
              role="status"
            />
          </div>
          <dl class="flex items-center gap-4 text-sm">
            <div>
              <dt class="sr-only">Best Score:</dt>
              <dd>Best Score: <strong>{{ getLabProgress(selectedLab)?.bestScore || 0 }}/{{ selectedLab.labMaxPoints }}</strong></dd>
            </div>
            <div>
              <dt class="sr-only">Attempts:</dt>
              <dd>Attempts: <strong>{{ getLabProgress(selectedLab)?.attemptCount || 0 }}</strong></dd>
            </div>
          </dl>
        </template>
        <template v-else>
          <p class="text-sm text-surface-500">You haven't attempted this lab yet.</p>
        </template>
      </div>

      <!-- Required badge -->
      <div
        v-if="selectedLab.isRequired"
        class="flex items-center gap-2 text-sm text-amber-600 dark:text-amber-400"
        role="note"
      >
        <i class="pi pi-exclamation-triangle" aria-hidden="true" />
        <span>Required to complete this module</span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <Button
          @click="viewLabDetails(selectedLab!)"
          label="View Details"
          icon="pi pi-info-circle"
          severity="secondary"
          :aria-label="`View full details for ${selectedLab?.labName}`"
        />
        <Button
          v-if="isEnrolled"
          @click="launchLab(selectedLab!)"
          :loading="launchingLab === selectedLab?.labTemplateId"
          :disabled="!!launchingLab"
          :label="getLabProgress(selectedLab!) ? 'Try Again' : 'Start Lab'"
          :icon="getLabProgress(selectedLab!) ? 'pi pi-replay' : 'pi pi-play'"
          :aria-label="`${getLabProgress(selectedLab!) ? 'Retry' : 'Start'} ${selectedLab?.labName}`"
        />
        <Button
          v-else
          @click="enroll"
          :loading="enrolling"
          :disabled="enrolling"
          label="Enroll to Start"
          icon="pi pi-plus"
          :aria-label="`Enroll in pathway to start ${selectedLab?.labName}`"
        />
      </div>
    </template>
  </Dialog>
</div>
```

---

## 4. Add Reduced Motion Support

**File:** `/Users/todd/Downloads/Claude/kootenai/web/src/components/pathway/PathwayRoadmap.vue`

Add to the `<script setup>` section after line 16:

```typescript
import { computed, ref, onMounted } from 'vue'

// Detect reduced motion preference
const prefersReducedMotion = ref(false)

onMounted(() => {
  prefersReducedMotion.value = window.matchMedia('(prefers-reduced-motion: reduce)').matches
})
```

Then update line 268 to respect this preference:

```vue
<div
  :class="[
    'w-16 h-16 rounded-full flex items-center justify-center shadow-lg transition-all duration-500',
    isPathwayComplete
      ? `bg-gradient-to-br from-emerald-500 to-green-600 scale-110 ${!prefersReducedMotion ? 'animate-pulse' : ''}`
      : 'bg-gradient-to-br from-surface-400 to-surface-500 dark:from-surface-600 dark:to-surface-700'
  ]"
>
```

---

## 5. Testing Checklist

After implementing these changes, test the following:

### Keyboard Navigation
- [ ] Can tab to all interactive elements
- [ ] Can activate buttons with Enter and Space
- [ ] Focus indicators are visible
- [ ] Tab order is logical
- [ ] Can escape from modal with Esc
- [ ] Skip link works and is visible on focus

### Screen Reader Testing (NVDA/JAWS/VoiceOver)
- [ ] Page structure is announced correctly
- [ ] All headings are read in order
- [ ] Interactive elements have clear labels
- [ ] Status changes are announced
- [ ] Progress bars announce current value
- [ ] Icons are hidden or have text alternatives
- [ ] Breadcrumb navigation is clear

### Visual Testing
- [ ] Focus indicators meet 3:1 contrast
- [ ] Text meets 4.5:1 contrast (AA)
- [ ] Large text meets 3:1 contrast (AA)
- [ ] Color is not the only differentiator
- [ ] Layout works at 200% zoom

### Automated Testing
```bash
# Install axe DevTools browser extension
# Or use axe-core programmatically

npm install --save-dev @axe-core/vue
```

---

## 6. Implementation Order

1. **First:** Add utility CSS (main.css)
2. **Second:** Fix PathwayRoadmap.vue keyboard navigation
3. **Third:** Fix PathwayInteractiveView.vue semantic structure
4. **Fourth:** Add reduced motion support
5. **Fifth:** Test with keyboard only
6. **Sixth:** Test with screen reader
7. **Seventh:** Fix any remaining issues found in testing

---

## Resources

- [WCAG 2.1 Quick Reference](https://www.w3.org/WAI/WCAG21/quickref/)
- [ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/)
- [WebAIM Keyboard Accessibility](https://webaim.org/techniques/keyboard/)
- [PrimeVue Accessibility](https://primevue.org/accessibility)
- [Vue.js Accessibility](https://vuejs.org/guide/best-practices/accessibility.html)

---

**End of Implementation Guide**
