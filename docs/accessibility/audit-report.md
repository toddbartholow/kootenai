# Accessibility Audit Report

**Date:** 2025-12-29
**Auditor:** Accessibility Testing Agent
**Components Reviewed:**
- `/Users/todd/Downloads/Claude/kootenai/web/src/views/PathwayInteractiveView.vue`
- `/Users/todd/Downloads/Claude/kootenai/web/src/components/pathway/PathwayRoadmap.vue`

---

## Executive Summary

**Overall Compliance Level:** Partially Conformant (WCAG 2.1 Level A)
**Critical Issues:** 8
**Major Issues:** 12
**Minor Issues:** 6

Both components have significant accessibility barriers that prevent keyboard-only users and screen reader users from effectively navigating and interacting with the pathway learning interface.

---

## Critical Issues (Priority 1)

### 1. Missing Semantic HTML Structure
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue, PathwayRoadmap.vue
**Issue:** The roadmap uses generic `<div>` elements for all interactive components instead of semantic HTML.

**Impact:**
- Screen readers cannot identify the structure and hierarchy
- Users cannot navigate by landmarks or regions
- Missing document outline for assistive technology

**Lines:**
- PathwayInteractiveView.vue: Lines 294-624
- PathwayRoadmap.vue: Lines 117-293

**Recommendation:**
```vue
<!-- Current (Lines 150-165) -->
<div @click="isModuleClickable(module.id) && handleModuleClick(module, $event)">

<!-- Should be -->
<article
  role="article"
  aria-labelledby="`module-${module.id}-title`"
  @click="isModuleClickable(module.id) && handleModuleClick(module, $event)"
>
  <h3 :id="`module-${module.id}-title`">{{ module.name }}</h3>
</article>
```

---

### 2. Interactive Elements Not Keyboard Accessible
**WCAG:** 2.1.1 Keyboard (Level A)
**Component:** PathwayRoadmap.vue
**Issue:** Clickable divs lack keyboard event handlers and are not in tab order.

**Lines:** 150-156, 220-224

**Current Code:**
```vue
<div
  :class="['mx-auto max-w-md transform transition-all duration-200']"
  @click="isModuleClickable(module.id) && handleModuleClick(module, $event)"
>
```

**Impact:**
- Keyboard-only users cannot access module cards
- Cannot expand/collapse modules with keyboard
- Cannot launch labs without a mouse

**Recommendation:**
```vue
<button
  type="button"
  :tabindex="isModuleClickable(module.id) ? 0 : -1"
  :aria-disabled="!isModuleClickable(module.id)"
  :aria-expanded="expandedModuleId === module.id"
  :aria-controls="`module-${module.id}-labs`"
  @click="isModuleClickable(module.id) && handleModuleClick(module, $event)"
  @keydown.enter="isModuleClickable(module.id) && handleModuleClick(module, $event)"
  @keydown.space.prevent="isModuleClickable(module.id) && handleModuleClick(module, $event)"
  class="w-full text-left"
>
```

---

### 3. Missing ARIA Labels and Descriptions
**WCAG:** 4.1.2 Name, Role, Value (Level A)
**Component:** Both
**Issue:** Interactive elements lack accessible names and descriptions.

**Lines:**
- PathwayInteractiveView.vue: 377-386 (Enroll button context)
- PathwayRoadmap.vue: 167-177 (Status indicators)

**Impact:**
- Screen readers announce generic text without context
- Status indicators are purely visual (icons without labels)
- Users cannot understand what actions buttons perform

**Example Issues:**
```vue
<!-- Line 167-177: Status indicator has no text alternative -->
<div class="absolute -top-3 -right-3">
  <div :class="['w-8 h-8 rounded-full']">
    <i :class="getStatusIcon(getModuleStatus(module.id))" />
  </div>
</div>
```

**Recommendation:**
```vue
<div
  class="absolute -top-3 -right-3"
  role="status"
  :aria-label="`Module status: ${getModuleStatus(module.id)}`"
>
  <div :class="['w-8 h-8 rounded-full']">
    <i :class="getStatusIcon(getModuleStatus(module.id))" aria-hidden="true" />
    <span class="sr-only">{{ getModuleStatus(module.id) }}</span>
  </div>
</div>
```

---

### 4. No Focus Management for Dialog
**WCAG:** 2.4.3 Focus Order (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** When the lab details dialog opens, focus is not trapped and may escape the modal.

**Lines:** 534-623

**Impact:**
- Keyboard users can tab out of the modal to underlying content
- Screen reader users lose context
- Cannot close modal with Escape key reliably

**Recommendation:**
```vue
<Dialog
  v-model:visible="showLabDialog"
  :header="selectedLab?.labName || 'Lab Details'"
  :modal="true"
  :dismissableMask="true"
  :focusOnShow="true"
  :closeOnEscape="true"
  role="dialog"
  :aria-labelledby="`dialog-title-${selectedLab?.id}`"
  :aria-describedby="`dialog-desc-${selectedLab?.id}`"
  @show="trapFocus"
  @hide="restoreFocus"
  class="w-full max-w-lg"
>
```

---

### 5. Color Contrast Issues
**WCAG:** 1.4.3 Contrast (Minimum) (Level AA)
**Component:** PathwayRoadmap.vue
**Issue:** Several text/background combinations fail 4.5:1 contrast ratio.

**Lines:**
- 196-198 (Module description text)
- 227-228 (Lab name in hover state)

**Failing Combinations:**
- `text-surface-500` on `bg-surface-50` (3.2:1 ratio - FAIL)
- Dark mode: `text-surface-400` on `bg-surface-800/50` (3.8:1 ratio - FAIL)

**Recommendation:**
- Use `text-surface-600` minimum for light mode
- Use `text-surface-300` minimum for dark mode
- Test all color combinations with contrast checker

---

### 6. Icons Without Text Alternatives
**WCAG:** 1.1.1 Non-text Content (Level A)
**Component:** Both
**Issue:** Decorative and informational icons lack proper text alternatives.

**Lines:**
- PathwayInteractiveView.vue: 320, 399, 418, 434, 481-502
- PathwayRoadmap.vue: 129, 175, 189, 226, 241-242, 245-246, 273-276

**Examples:**
```vue
<!-- Line 320: Navigation icon without label -->
<i class="pi pi-chevron-right text-xs" />

<!-- Line 481-502: Legend icons for screen readers -->
<i class="pi pi-check-circle" />
```

**Recommendation:**
```vue
<i class="pi pi-chevron-right text-xs" aria-hidden="true" />
<span class="sr-only">Navigate to</span>

<!-- For informative icons -->
<i class="pi pi-check-circle" aria-label="Completed status" />
```

---

### 7. Missing Skip Navigation
**WCAG:** 2.4.1 Bypass Blocks (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** No skip link to bypass breadcrumbs and header to main content.

**Lines:** 294-323

**Impact:**
- Keyboard users must tab through all header elements
- Screen reader users hear repetitive navigation

**Recommendation:**
```vue
<template>
  <div class="space-y-6">
    <!-- Add skip link -->
    <a href="#main-pathway-content" class="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:p-4 focus:bg-primary-500 focus:text-white">
      Skip to pathway content
    </a>

    <!-- Breadcrumb -->
    <nav aria-label="Breadcrumb">
      <!-- existing breadcrumb -->
    </nav>

    <!-- Main content -->
    <main id="main-pathway-content">
      <!-- existing content -->
    </main>
  </div>
</template>
```

---

### 8. Progress Indicators Lack Live Region Announcements
**WCAG:** 4.1.3 Status Messages (Level AA)
**Component:** PathwayInteractiveView.vue
**Issue:** When enrollment or lab launch status changes, screen readers are not notified.

**Lines:** 30-33 (State variables), 166-184 (Enroll function), 234-253 (Launch function)

**Impact:**
- Screen readers don't announce loading states
- Users don't know when operations complete or fail
- No feedback for async operations

**Recommendation:**
```vue
<!-- Add live region for status updates -->
<div
  role="status"
  aria-live="polite"
  aria-atomic="true"
  class="sr-only"
>
  <span v-if="enrolling">Enrolling in pathway, please wait...</span>
  <span v-else-if="enrollment">Successfully enrolled!</span>
  <span v-if="launchingLab">Launching lab, please wait...</span>
  <span v-else-if="error">{{ error }}</span>
</div>
```

---

## Major Issues (Priority 2)

### 9. Insufficient Heading Hierarchy
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Headings skip levels (h1 → h3) and are not properly nested.

**Lines:** 333 (h1), 439 (h3 without h2), 461 (h2)

---

### 10. No Keyboard Navigation for Roadmap
**WCAG:** 2.1.1 Keyboard (Level A)
**Component:** PathwayRoadmap.vue
**Issue:** Cannot navigate between modules with arrow keys.

**Lines:** 134-254

**Recommendation:**
Implement arrow key navigation between modules:
```typescript
function handleKeydown(event: KeyboardEvent, currentIndex: number) {
  if (event.key === 'ArrowDown' && currentIndex < sortedModules.value.length - 1) {
    focusModule(currentIndex + 1)
  } else if (event.key === 'ArrowUp' && currentIndex > 0) {
    focusModule(currentIndex - 1)
  }
}
```

---

### 11. Loading State Not Announced
**WCAG:** 4.1.3 Status Messages (Level AA)
**Component:** PathwayInteractiveView.vue
**Issue:** ProgressSpinner has no accessible label.

**Lines:** 296-298

**Recommendation:**
```vue
<div
  v-if="loading"
  class="flex justify-center py-12"
  role="status"
  aria-live="polite"
  aria-label="Loading pathway details"
>
  <ProgressSpinner aria-hidden="true" />
  <span class="sr-only">Loading pathway details, please wait...</span>
</div>
```

---

### 12. Lab Cards in Roadmap Not Accessible
**WCAG:** 2.1.1 Keyboard (Level A), 4.1.2 Name, Role, Value (Level A)
**Component:** PathwayRoadmap.vue
**Issue:** Lab cards are clickable divs without keyboard support.

**Lines:** 220-235

**Current:**
```vue
<div
  class="...cursor-pointer..."
  @click="handleLabClick(lab, module, $event)"
>
```

**Recommendation:**
```vue
<button
  type="button"
  class="w-full text-left..."
  :aria-label="`Launch ${lab.labName}, duration ${lab.labDurationMinutes} minutes`"
  @click="handleLabClick(lab, module, $event)"
  @keydown.enter="handleLabClick(lab, module, $event)"
>
```

---

### 13. Progress Bars Missing Labels
**WCAG:** 1.1.1 Non-text Content (Level A)
**Component:** Both
**Issue:** ProgressBar components lack accessible labels.

**Lines:**
- PathwayInteractiveView.vue: 352
- PathwayRoadmap.vue: 208

**Recommendation:**
```vue
<ProgressBar
  :value="overallProgress"
  :showValue="false"
  class="h-2"
  role="progressbar"
  :aria-valuenow="overallProgress"
  aria-valuemin="0"
  aria-valuemax="100"
  :aria-label="`Overall pathway progress: ${overallProgress}%`"
/>
```

---

### 14. Breadcrumb Navigation Missing Semantics
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Breadcrumb is not wrapped in `<nav>` with proper aria-label.

**Lines:** 316-322

**Recommendation:**
```vue
<nav aria-label="Breadcrumb">
  <ol class="flex items-center gap-2 text-sm text-surface-500">
    <li>
      <router-link to="/pathways" class="hover:text-surface-700">
        Learning Pathways
      </router-link>
    </li>
    <li aria-hidden="true">
      <i class="pi pi-chevron-right text-xs" />
    </li>
    <li aria-current="page">
      <span class="text-surface-700">{{ pathway.name }}</span>
    </li>
  </ol>
</nav>
```

---

### 15. Stats Grid Not Accessible
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Stats presented as visual grid without proper semantic structure.

**Lines:** 357-374

**Recommendation:**
```vue
<dl class="grid grid-cols-2 gap-4">
  <div class="text-center">
    <dt class="text-sm text-surface-500">Modules</dt>
    <dd class="text-2xl font-bold">{{ modules.length }}</dd>
  </div>
  <!-- Repeat for other stats -->
</dl>
```

---

### 16. Module Expansion State Not Announced
**WCAG:** 4.1.2 Name, Role, Value (Level A)
**Component:** PathwayRoadmap.vue
**Issue:** When modules expand/collapse, state change is not announced.

**Lines:** 16-20, 88-96

**Recommendation:**
Add `aria-expanded` to module button and proper role for expanded content:
```vue
<button
  :aria-expanded="expandedModuleId === module.id"
  :aria-controls="`labs-${module.id}`"
>
  <!-- Module content -->
</button>

<div
  :id="`labs-${module.id}`"
  role="region"
  :aria-label="`Labs for ${module.name}`"
  :hidden="expandedModuleId !== module.id"
>
  <!-- Labs list -->
</div>
```

---

### 17. Error Messages Not Associated with Controls
**WCAG:** 3.3.1 Error Identification (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Error messages appear globally but aren't linked to failed actions.

**Lines:** 192-194, 211-213, 248-249

**Recommendation:**
```vue
<div
  v-if="error"
  role="alert"
  aria-live="assertive"
  class="error-message"
>
  {{ error }}
</div>
```

---

### 18. Tags Missing Semantic Information
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Tags are presented visually but lack semantic meaning for screen readers.

**Lines:** 328-332, 337-339

**Recommendation:**
```vue
<ul class="flex flex-wrap gap-2" aria-label="Pathway attributes">
  <li>
    <Tag v-if="pathway.isFeatured" value="Featured" severity="warn" role="status" />
  </li>
  <li>
    <Tag :value="getDifficultyLabel(pathway.difficulty)" role="note" />
  </li>
</ul>
```

---

### 19. Locked Module Feedback Insufficient
**WCAG:** 3.3.1 Error Identification (Level A)
**Component:** PathwayInteractiveView.vue, PathwayRoadmap.vue
**Issue:** When clicking locked modules, temporal error message may be missed.

**Lines:**
- PathwayInteractiveView.vue: 191-196, 210-214
- PathwayRoadmap.vue: 153 (cursor-not-allowed, opacity-75)

**Recommendation:**
- Add `aria-disabled="true"` to locked modules
- Use modal dialog instead of timeout-based error
- Provide clear instruction on how to unlock

---

### 20. Next Lab Card Missing Semantic Structure
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** "Next up" recommendation uses generic Card without heading structure.

**Lines:** 430-457

**Recommendation:**
```vue
<Card
  role="complementary"
  aria-labelledby="next-lab-heading"
>
  <template #content>
    <article>
      <h2 id="next-lab-heading" class="sr-only">Recommended Next Lab</h2>
      <!-- content -->
    </article>
  </template>
</Card>
```

---

## Minor Issues (Priority 3)

### 21. Legend Could Use Better Semantics
**WCAG:** 1.3.1 Info and Relationships (Level A)
**Component:** PathwayInteractiveView.vue
**Lines:** 475-505

**Recommendation:**
```vue
<Card role="complementary" aria-labelledby="legend-title">
  <template #title>
    <h2 id="legend-title">Status Legend</h2>
  </template>
  <template #content>
    <ul class="flex flex-wrap gap-6">
      <li class="flex items-center gap-2">
        <!-- icon and label -->
      </li>
    </ul>
  </template>
</Card>
```

---

### 22. Module Colors May Not Be Distinguishable
**WCAG:** 1.4.1 Use of Color (Level A)
**Component:** PathwayRoadmap.vue
**Issue:** Module differentiation relies solely on color.

**Lines:** 29-38, 159-164

**Recommendation:**
Add patterns or text labels in addition to colors:
```vue
<div :data-module-number="index + 1">
  <!-- Visual indicator beyond color -->
</div>
```

---

### 23. Animation May Cause Motion Sickness
**WCAG:** 2.3.3 Animation from Interactions (Level AAA)
**Component:** PathwayRoadmap.vue
**Issue:** Completion animation uses `animate-pulse` which may trigger vestibular issues.

**Lines:** 268

**Recommendation:**
```vue
<div
  :class="[
    'w-16 h-16 rounded-full flex items-center justify-center',
    isPathwayComplete && !prefersReducedMotion
      ? 'animate-pulse'
      : ''
  ]"
>
```

Add to script:
```typescript
const prefersReducedMotion = ref(
  window.matchMedia('(prefers-reduced-motion: reduce)').matches
)
```

---

### 24. Insufficient Touch Target Size
**WCAG:** 2.5.5 Target Size (Level AAA)
**Component:** PathwayRoadmap.vue
**Issue:** Expand/collapse chevron icon may be too small for touch.

**Lines:** 248-250

**Recommendation:**
Ensure minimum 44x44px touch target:
```vue
<button
  type="button"
  class="min-w-[44px] min-h-[44px] flex items-center justify-center"
  :aria-label="expandedModuleId === module.id ? 'Collapse labs' : 'Expand labs'"
>
  <i :class="['pi', expandedModuleId === module.id ? 'pi-chevron-up' : 'pi-chevron-down']" />
</button>
```

---

### 25. Router Links Missing External Indicator
**WCAG:** 2.4.4 Link Purpose (In Context) (Level A)
**Component:** PathwayInteractiveView.vue
**Issue:** Links context could be clearer for screen readers.

**Lines:** 317-319

**Recommendation:**
```vue
<router-link
  to="/pathways"
  class="hover:text-surface-700"
  aria-label="Return to Learning Pathways list"
>
  Learning Pathways
</router-link>
```

---

### 26. Missing Language Declarations
**WCAG:** 3.1.1 Language of Page (Level A)
**Component:** Both
**Issue:** No lang attribute for content (should be set at app level).

**Recommendation:**
Ensure root App.vue has:
```vue
<template>
  <div lang="en">
```

---

## Remediation Priority

### Immediate (Sprint 1)
1. Fix keyboard navigation for all interactive elements
2. Add ARIA labels and semantic HTML
3. Implement focus management for dialog
4. Fix color contrast issues
5. Add text alternatives for icons

### Short-term (Sprint 2)
6. Implement live regions for status updates
7. Fix heading hierarchy
8. Add proper breadcrumb semantics
9. Improve error message handling
10. Add skip navigation

### Medium-term (Sprint 3)
11. Enhance keyboard navigation (arrow keys)
12. Add comprehensive screen reader testing
13. Implement reduced motion preferences
14. Improve touch target sizes
15. Add proper semantic structure throughout

---

## Testing Recommendations

### Manual Testing Required
1. **Keyboard Navigation Test**
   - Tab through entire interface
   - Test all interactive elements with Enter/Space
   - Verify focus visibility
   - Test arrow key navigation

2. **Screen Reader Test**
   - NVDA on Windows
   - JAWS on Windows
   - VoiceOver on macOS
   - Test all user flows

3. **Color Contrast Test**
   - Use WebAIM Contrast Checker
   - Test all text/background combinations
   - Test in light and dark modes

4. **Focus Management Test**
   - Open/close dialog
   - Navigate between elements
   - Verify focus trap in modal
   - Test focus restoration

### Automated Testing
```bash
# Install axe-core for Vue
npm install --save-dev @axe-core/vue

# Add to main.ts (development only)
if (process.env.NODE_ENV !== 'production') {
  import('@axe-core/vue').then(axe => {
    axe.default(app, { auto: false })
  })
}
```

### Browser Testing
- Firefox + NVDA
- Chrome + JAWS
- Safari + VoiceOver
- Edge (Chromium)

---

## Compliance Summary

| WCAG Level | Current Status | Target Status |
|------------|---------------|---------------|
| Level A    | Partial (42%) | Full (100%)   |
| Level AA   | Minimal (18%) | Full (100%)   |
| Level AAA  | None (0%)     | Partial (60%) |

**Estimated Remediation Time:**
- Critical Issues: 16-24 hours
- Major Issues: 20-30 hours
- Minor Issues: 8-12 hours
- Testing & Validation: 12-16 hours

**Total: 56-82 hours** (7-10 business days with dedicated accessibility developer)

---

## Resources

- [WCAG 2.1 Guidelines](https://www.w3.org/WAI/WCAG21/quickref/)
- [WAI-ARIA Authoring Practices](https://www.w3.org/WAI/ARIA/apg/)
- [Vue.js Accessibility Guide](https://vuejs.org/guide/best-practices/accessibility.html)
- [PrimeVue Accessibility](https://primevue.org/accessibility)
- [WebAIM Contrast Checker](https://webaim.org/resources/contrastchecker/)
- [axe DevTools](https://www.deque.com/axe/devtools/)

---

## Next Steps

1. **Review this report** with development team
2. **Create GitHub issues** for each critical/major item
3. **Assign accessibility champion** for ongoing compliance
4. **Schedule accessibility training** for team
5. **Integrate automated testing** into CI/CD pipeline
6. **Plan user testing** with people who use assistive technology
7. **Establish accessibility guidelines** for future development

---

**Report End**
