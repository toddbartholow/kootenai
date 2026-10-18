<template>
    <Button
        unstyled
        :pt="theme"
        :ptOptions="{
            mergeProps: ptViewMerge
        }"
    >
        <template v-for="(_, slotName) in $slots" #[slotName]="slotProps">
            <slot :name="slotName" v-bind="slotProps ?? {}" />
        </template>
    </Button>
</template>

<script setup lang="ts">
import Button, { type ButtonPassThroughOptions, type ButtonProps } from 'primevue/button';
import { ref } from 'vue';
import { ptViewMerge } from './utils';

interface Props extends /* @vue-ignore */ ButtonProps {}
defineProps<Props>();

const theme = ref<ButtonPassThroughOptions>({
    root: `inline-flex cursor-pointer select-none items-center justify-center overflow-hidden relative
        px-3 py-2 gap-2 rounded-md disabled:pointer-events-none disabled:opacity-60 transition-colors duration-200
        bg-primary-500 enabled:hover:bg-primary-600 enabled:active:bg-primary-700 text-white
        border border-primary-500 enabled:hover:border-primary-600 enabled:active:border-primary-700
        dark:bg-primary-600 dark:enabled:hover:bg-primary-500 dark:enabled:active:bg-primary-400 dark:text-white
        dark:border-primary-600 dark:enabled:hover:border-primary-500 dark:enabled:active:border-primary-400
        focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-primary-500
        p-vertical:flex-col p-fluid:w-full p-fluid:p-icon-only:w-10
        p-icon-only:w-10 p-icon-only:px-0 p-icon-only:gap-0
        p-icon-only:p-rounded:rounded-full p-icon-only:p-rounded:h-10
        p-small:text-sm p-small:px-[0.625rem] p-small:py-[0.375rem]
        p-large:text-[1.125rem] p-large:px-[0.875rem] p-large:py-[0.625rem]
        p-raised:shadow-sm p-rounded:rounded-[2rem]
        p-outlined:bg-transparent enabled:hover:p-outlined:bg-primary-50 enabled:active:p-outlined:bg-primary-100
        p-outlined:border-primary-300 enabled:hover:p-outlined:border-primary-400 enabled:active:p-outlined:border-primary-500
        p-outlined:text-primary-600 enabled:hover:p-outlined:text-primary-700 enabled:active:p-outlined:text-primary-800
        dark:p-outlined:bg-transparent dark:enabled:hover:p-outlined:bg-primary-900/30 dark:enabled:active:p-outlined:bg-primary-900/50
        dark:p-outlined:border-primary-500 dark:enabled:hover:p-outlined:border-primary-400 dark:enabled:active:p-outlined:border-primary-300
        dark:p-outlined:text-primary-400 dark:enabled:hover:p-outlined:text-primary-300 dark:enabled:active:p-outlined:text-primary-200
        p-success:bg-green-500 p-success:border-green-500 p-success:text-white
        enabled:hover:p-success:bg-green-600 enabled:hover:p-success:border-green-600
        enabled:active:p-success:bg-green-700 enabled:active:p-success:border-green-700
        dark:p-success:bg-green-600 dark:p-success:border-green-600 dark:p-success:text-white
        dark:enabled:hover:p-success:bg-green-500 dark:enabled:hover:p-success:border-green-500
        dark:enabled:active:p-success:bg-green-400 dark:enabled:active:p-success:border-green-400
        p-success:p-outlined:bg-transparent p-success:p-outlined:border-green-500 p-success:p-outlined:text-green-600
        enabled:hover:p-success:p-outlined:bg-green-50 enabled:hover:p-success:p-outlined:border-green-600 enabled:hover:p-success:p-outlined:text-green-700
        enabled:active:p-success:p-outlined:bg-green-100 enabled:active:p-success:p-outlined:border-green-700 enabled:active:p-success:p-outlined:text-green-800
        dark:p-success:p-outlined:bg-transparent dark:p-success:p-outlined:border-green-400 dark:p-success:p-outlined:text-green-400
        dark:enabled:hover:p-success:p-outlined:bg-green-500/10 dark:enabled:hover:p-success:p-outlined:border-green-300 dark:enabled:hover:p-success:p-outlined:text-green-300
        dark:enabled:active:p-success:p-outlined:bg-green-500/20 dark:enabled:active:p-success:p-outlined:border-green-300 dark:enabled:active:p-success:p-outlined:text-green-200
        p-danger:bg-red-500 p-danger:border-red-500 p-danger:text-white
        enabled:hover:p-danger:bg-red-600 enabled:hover:p-danger:border-red-600
        enabled:active:p-danger:bg-red-700 enabled:active:p-danger:border-red-700
        dark:p-danger:bg-red-600 dark:p-danger:border-red-600 dark:p-danger:text-white
        dark:enabled:hover:p-danger:bg-red-500 dark:enabled:hover:p-danger:border-red-500
        dark:enabled:active:p-danger:bg-red-400 dark:enabled:active:p-danger:border-red-400
        p-danger:p-outlined:bg-transparent p-danger:p-outlined:border-red-500 p-danger:p-outlined:text-red-600
        enabled:hover:p-danger:p-outlined:bg-red-50 enabled:hover:p-danger:p-outlined:border-red-600 enabled:hover:p-danger:p-outlined:text-red-700
        enabled:active:p-danger:p-outlined:bg-red-100 enabled:active:p-danger:p-outlined:border-red-700 enabled:active:p-danger:p-outlined:text-red-800
        dark:p-danger:p-outlined:bg-transparent dark:p-danger:p-outlined:border-red-400 dark:p-danger:p-outlined:text-red-400
        dark:enabled:hover:p-danger:p-outlined:bg-red-500/10 dark:enabled:hover:p-danger:p-outlined:border-red-300 dark:enabled:hover:p-danger:p-outlined:text-red-300
        dark:enabled:active:p-danger:p-outlined:bg-red-500/20 dark:enabled:active:p-danger:p-outlined:border-red-300 dark:enabled:active:p-danger:p-outlined:text-red-200
        p-text:bg-transparent enabled:hover:p-text:bg-primary-50 enabled:active:p-text:bg-primary-100
        p-text:border-transparent enabled:hover:p-text:border-transparent enabled:active:p-text:border-transparent
        p-text:text-primary-600 enabled:hover:p-text:text-primary-700 enabled:active:p-text:text-primary-800
        dark:p-text:bg-transparent dark:enabled:hover:p-text:bg-surface-800 dark:enabled:active:p-text:bg-surface-700
        dark:p-text:border-transparent dark:enabled:hover:p-text:border-transparent dark:enabled:active:p-text:border-transparent
        dark:p-text:text-primary-400 dark:enabled:hover:p-text:text-primary-300 dark:enabled:active:p-text:text-primary-200
        p-secondary:bg-surface-100 enabled:hover:p-secondary:bg-surface-200 enabled:active:p-secondary:bg-surface-300
        p-secondary:border-surface-200 enabled:hover:p-secondary:border-surface-300 enabled:active:p-secondary:border-surface-400
        p-secondary:text-surface-700 enabled:hover:p-secondary:text-surface-800 enabled:active:p-secondary:text-surface-900
        dark:p-secondary:bg-surface-800 dark:enabled:hover:p-secondary:bg-surface-700 dark:enabled:active:p-secondary:bg-surface-600
        dark:p-secondary:border-surface-700 dark:enabled:hover:p-secondary:border-surface-600 dark:enabled:active:p-secondary:border-surface-500
        dark:p-secondary:text-surface-100 dark:enabled:hover:p-secondary:text-surface-0 dark:enabled:active:p-secondary:text-surface-0
    `,
    loadingIcon: `animate-spin`,
    icon: `p-right:order-1 p-bottom:order-2`,
    label: `font-medium p-icon-only:invisible p-icon-only:w-0
        p-small:text-sm p-large:text-[1.125rem]`,
    pcBadge: {
        root: `min-w-4 h-4 leading-4 bg-white dark:bg-surface-100 rounded-full text-primary-600 dark:text-primary-500 text-xs font-bold`
    }
});
</script>
