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
                <div v-show="isBlurred" class="text-output-area__show">
                    <v-btn
                        class="bg-background"
                        variant="outlined"
                        color="default"
                        :prepend-icon="mdiLockOutline"
                        @click="isBlurred = false"
                    >
                        Show {{ label }}
                    </v-btn>
                </div>
            </v-fade-transition>
        </template>

        <template v-if="showCopy" #append-inner>
            <input-copy-button :value="value" :tooltip-disabled="tooltipDisabled" />
        </template>
    </v-textarea>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VTextarea, VFadeTransition, VBtn } from 'vuetify/components';
import { mdiLockOutline } from '@mdi/js';

import InputCopyButton from '@/components/InputCopyButton.vue';

const props = defineProps<{
    label: string;
    value: string;
    centerText?: boolean;
    tooltipDisabled?: boolean;
    showCopy?: boolean;
}>();

const isBlurred = ref<boolean>(true);
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
