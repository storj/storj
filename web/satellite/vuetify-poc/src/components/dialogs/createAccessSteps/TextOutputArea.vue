// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-textarea
        class="text-output-area"
        :class="{
            'text-output-area--center-text': centerText,
            'text-output-area--unblur': !isBlurred,
        }"
        variant="solo-filled"
        :label="label"
        :model-value="value"
        rows="1"
        auto-grow
        no-resize
        readonly
        hide-details
        flat
    >
        <template #prepend-inner>
            <v-fade-transition>
                <div v-show="isBlurred" class="text-output-area__show">
                    <v-btn
                        class="bg-background"
                        variant="outlined"
                        color="default"
                        size="small"
                        prepend-icon="mdi-lock-outline"
                        @click="isBlurred = false"
                    >
                        Show {{ label }}
                    </v-btn>
                </div>
            </v-fade-transition>
        </template>

        <template v-if="showCopy" #append-inner>
            <v-tooltip v-model="isTooltip" location="start">
                <template #activator="{ props: activatorProps }">
                    <v-btn
                        v-bind="activatorProps"
                        :icon="justCopied ? 'mdi-check' : 'mdi-content-copy'"
                        variant="text"
                        density="compact"
                        :color="justCopied ? 'success' : 'default'"
                        @click="onCopy"
                    />
                </template>
                {{ justCopied ? 'Copied!' : 'Copy' }}
            </v-tooltip>
        </template>
    </v-textarea>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { VTextarea, VFadeTransition, VBtn, VTooltip } from 'vuetify/components';

import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

const props = defineProps<{
    label: string;
    value: string;
    centerText?: boolean;
    tooltipDisabled?: boolean;
    showCopy?: boolean;
}>();

const isBlurred = ref<boolean>(true);

const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const justCopied = computed<boolean>(() => copiedTimeout.value !== null);

const isTooltip = (() => {
    const internal = ref<boolean>(false);
    return computed<boolean>({
        get: () => (internal.value || justCopied.value) && !props.tooltipDisabled,
        set: v => internal.value = v,
    });
})();

const analyticsStore = useAnalyticsStore();

/**
 * Saves value to clipboard.
 */
function onCopy(): void {
    navigator.clipboard.writeText(props.value);
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);

    if (copiedTimeout.value) clearTimeout(copiedTimeout.value);
    copiedTimeout.value = setTimeout(() => {
        copiedTimeout.value = null;
    }, 750);
}
</script>

<style scoped lang="scss">
.text-output-area {

    &__show {
        position: absolute;
        z-index: 1;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: default;
    }

    :deep(textarea) {
        font-family: monospace;
    }

    :deep(.v-field__field), :deep(.v-field__append-inner) {
        filter: blur(10px);
    }

    &--unblur {

        :deep(.v-field__field), :deep(.v-field__append-inner) {
            filter: none;
            transition: filter 0.25s ease;
        }
    }

    &--center-text :deep(textarea) {
        text-align: center;
    }
}
</style>
