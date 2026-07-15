// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        ref="container"
        class="d-flex align-center justify-center"
    />
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';

import { loadTurnstile } from '@/utils/turnstile';

const props = defineProps<{
    siteKey: string;
}>();

const emit = defineEmits<{
    verify: [token: string];
    error: [];
    expired: [];
}>();

const container = ref<HTMLElement | null>(null);
const widgetId = ref<string | null>(null);

/**
 * Renders the Turnstile widget in deferred (execute) mode.
 */
async function render(): Promise<void> {
    const turnstile = await loadTurnstile();
    if (!container.value) {
        return;
    }
    widgetId.value = turnstile.render(container.value, {
        sitekey: props.siteKey,
        execution: 'execute',
        appearance: 'interaction-only',
        callback: (token: string) => emit('verify', token),
        'error-callback': () => emit('error'),
        'expired-callback': () => emit('expired'),
        'timeout-callback': () => emit('expired'),
    });
}

/**
 * Runs the Turnstile challenge.
 */
async function execute(): Promise<void> {
    try {
        const turnstile = await loadTurnstile();
        if (widgetId.value === null) {
            await render();
        }
        if (widgetId.value !== null) {
            turnstile.execute(widgetId.value);
        }
    } catch {
        emit('error');
    }
}

/**
 * Clears the current challenge state.
 */
function reset(): void {
    if (widgetId.value !== null && window.turnstile) {
        window.turnstile.reset(widgetId.value);
    }
}

defineExpose({ execute, reset });

onMounted(() => {
    loadTurnstile().catch(() => emit('error'));
});

onBeforeUnmount(() => {
    if (widgetId.value !== null && window.turnstile) {
        window.turnstile.remove(widgetId.value);
    }
});
</script>
