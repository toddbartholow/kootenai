import { ref, computed, onMounted, onUnmounted, type Ref } from 'vue'

export interface VirtualListOptions {
  /** Number of items to render beyond the visible viewport */
  overscan?: number
  /** Item height in pixels (for calculations) */
  itemHeight?: number
  /** Container height in pixels (auto-detected if not provided) */
  containerHeight?: number
  /** Enable/disable virtual scrolling */
  enabled?: boolean
}

export interface VirtualListReturn<T> {
  /** Ref to attach to the scrollable container */
  containerRef: Ref<HTMLElement | null>
  /** Items that should currently be rendered */
  visibleItems: Ref<T[]>
  /** Starting index of visible items in the original array */
  startIndex: Ref<number>
  /** Total height of all items (for scroll container sizing) */
  totalHeight: Ref<number>
  /** Offset to position visible items correctly */
  offsetY: Ref<number>
  /** Whether virtual scrolling is active */
  isVirtual: Ref<boolean>
  /** Scroll to a specific index */
  scrollToIndex: (index: number) => void
}

/**
 * Composable for virtual scrolling of large lists.
 *
 * Best for lists with many items (100+) that have a consistent item height.
 * For grid layouts, consider using intersection observer-based lazy loading instead.
 *
 * @example
 * ```vue
 * <script setup>
 * const items = ref([...]) // Large array of items
 * const { containerRef, visibleItems, offsetY, totalHeight } = useVirtualList(items, {
 *   itemHeight: 80,
 *   overscan: 5
 * })
 * </script>
 *
 * <template>
 *   <div ref="containerRef" class="overflow-auto h-96">
 *     <div :style="{ height: `${totalHeight}px`, position: 'relative' }">
 *       <div :style="{ transform: `translateY(${offsetY}px)` }">
 *         <div v-for="item in visibleItems" :key="item.id">
 *           {{ item.name }}
 *         </div>
 *       </div>
 *     </div>
 *   </div>
 * </template>
 * ```
 */
export function useVirtualList<T>(
  items: Ref<T[]>,
  options: VirtualListOptions = {}
): VirtualListReturn<T> {
  const {
    overscan = 5,
    itemHeight = 50,
    containerHeight: providedContainerHeight,
    enabled = true
  } = options

  const containerRef = ref<HTMLElement | null>(null)
  const scrollTop = ref(0)
  const containerHeight = ref(providedContainerHeight || 400)

  // Calculate visible range
  const startIndex = computed(() => {
    if (!enabled) return 0
    const start = Math.floor(scrollTop.value / itemHeight) - overscan
    return Math.max(0, start)
  })

  const endIndex = computed(() => {
    if (!enabled) return items.value.length
    const visibleCount = Math.ceil(containerHeight.value / itemHeight)
    const end = startIndex.value + visibleCount + (overscan * 2)
    return Math.min(items.value.length, end)
  })

  const visibleItems = computed(() => {
    if (!enabled) return items.value
    return items.value.slice(startIndex.value, endIndex.value)
  })

  const totalHeight = computed(() => {
    if (!enabled) return 0
    return items.value.length * itemHeight
  })

  const offsetY = computed(() => {
    if (!enabled) return 0
    return startIndex.value * itemHeight
  })

  const isVirtual = computed(() => enabled && items.value.length > 50)

  function handleScroll() {
    if (containerRef.value) {
      scrollTop.value = containerRef.value.scrollTop
    }
  }

  function updateContainerHeight() {
    if (containerRef.value && !providedContainerHeight) {
      containerHeight.value = containerRef.value.clientHeight
    }
  }

  function scrollToIndex(index: number) {
    if (containerRef.value) {
      containerRef.value.scrollTop = index * itemHeight
    }
  }

  onMounted(() => {
    if (containerRef.value) {
      containerRef.value.addEventListener('scroll', handleScroll, { passive: true })
      updateContainerHeight()

      // Update on resize
      const resizeObserver = new ResizeObserver(updateContainerHeight)
      resizeObserver.observe(containerRef.value)

      onUnmounted(() => {
        if (containerRef.value) {
          containerRef.value.removeEventListener('scroll', handleScroll)
          resizeObserver.disconnect()
        }
      })
    }
  })

  return {
    containerRef,
    visibleItems,
    startIndex,
    totalHeight,
    offsetY,
    isVirtual,
    scrollToIndex
  }
}

/**
 * Composable for lazy loading items as they come into view.
 * Better suited for grid layouts where items have varying or unknown heights.
 *
 * @example
 * ```vue
 * <script setup>
 * const items = ref([...])
 * const { isVisible, observeElement } = useLazyList(items)
 * </script>
 *
 * <template>
 *   <div v-for="(item, index) in items" :key="item.id" :ref="el => observeElement(el, index)">
 *     <template v-if="isVisible(index)">
 *       <!-- Full content -->
 *     </template>
 *     <template v-else>
 *       <!-- Placeholder -->
 *     </template>
 *   </div>
 * </template>
 * ```
 */
export function useLazyList<T>(_items: Ref<T[]>, rootMargin = '200px') {
  const visibleIndices = ref(new Set<number>())
  const observers = new Map<number, IntersectionObserver>()

  function isVisible(index: number): boolean {
    return visibleIndices.value.has(index)
  }

  function observeElement(el: HTMLElement | null, index: number) {
    if (!el) {
      // Element unmounted, cleanup observer
      const observer = observers.get(index)
      if (observer) {
        observer.disconnect()
        observers.delete(index)
      }
      return
    }

    // Create observer for this element
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            visibleIndices.value.add(index)
          }
          // Keep items visible once they've been seen (for smooth scrolling)
        })
      },
      { rootMargin }
    )

    observer.observe(el)
    observers.set(index, observer)
  }

  onUnmounted(() => {
    observers.forEach(observer => observer.disconnect())
    observers.clear()
  })

  return {
    isVisible,
    observeElement,
    visibleIndices
  }
}

export default useVirtualList
