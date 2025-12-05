// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-textarea
        class="text-output-area"
        :class="{ 'text-output-area--unblur': !isBlurred }"
        variant="solo-filled"
        :label="label"
        :model-value="value"
        rounded="xlg"
        rows="1"
        auto-grow
        no-resize
        readonly
        hide-details
        flat
    >
        <template #prepend-inner>
            <v-fade-transition>
                <div v-show="isBlurred" class="text-output-area__show pl-5 pr-3">
                    <p class="text-caption w-100">{{ label }}</p>
                    <v-btn
                        class="bg-background mx-2"
                        variant="outlined"
                        color="default"
                        size="small"
                        :prepend-icon="LockKeyhole"
                        @click="isBlurred = false"
                    >
                        Show
                    </v-btn>
                    <input-copy-button :value="value" :tooltip-disabled="true" />
                </div>
            </v-fade-transition>
        </template>

        <template #append-inner>
            <input-copy-button :value="value" />
        </template>
    </v-textarea>
</template>

<script setup lang="ts">
import { VTextarea, VFadeTransition, VBtn } from 'vuetify/components';
import { LockKeyhole } from 'lucide-vue-next';

import InputCopyButton from '@/components/InputCopyButton.vue';

defineProps<{
    label: string;
    value: string;
}>();

const isBlurred = defineModel<boolean>('isBlurred', { default: true });
</script>

<style scoped lang="scss">
.text-output-area {

    &__show {
        position: absolute;
        z-index: 1;
        inset: 0;
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: default;
    }

    :deep(textarea) {
        font-family: monospace;
        font-size: 14px;
        margin-bottom: 8px;
        margin-top: 8px;
    }

    :deep(.v-field__field), :deep(.v-field__append-inner) {
        filter: blur(50px);
    }

    :deep(.v-field-label--floating) {
        top: 10px !important;
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
