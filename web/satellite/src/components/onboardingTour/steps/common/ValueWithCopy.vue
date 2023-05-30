// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="value-copy">
        <p class="value-copy__value" :title="value" :aria-roledescription="roleDescription">{{ value }}</p>
        <VButton
            class="value-copy__button"
            label="Copy"
            width="66px"
            height="30px"
            :is-blue-white="true"
            :on-press="onCopyClick"
        />
    </div>
</template>

<script setup lang="ts">
import { useNotify } from '@/utils/hooks';

import VButton from '@/components/common/VButton.vue';

const notify = useNotify();

const props = withDefaults(defineProps<{
    value: string;
    label: string;
    roleDescription: string;
}>(), {
    value: '',
    label: '',
    roleDescription: '',
});

/**
 * Holds on copy button click logic.
 * Copies value to clipboard.
 */
function onCopyClick(): void {
    navigator.clipboard.writeText(props.value);
    notify.success(`${props.label} was copied successfully`);
}
</script>

<style scoped lang="scss">
    .value-copy {
        display: flex;
        align-items: center;
        padding: 12px 25px;
        background: #eff0f7;
        border-radius: 10px;
        max-width: calc(100% - 50px);

        @media screen and (width <= 450px) {
            padding: 12px;
            max-width: calc(100% - 24px);
        }

        &__value {
            font-size: 16px;
            line-height: 28px;
            color: #384b65;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }

        &__button {
            margin-left: 32px;
            min-width: 66px;

            @media screen and (width <= 450px) {
                margin-left: 12px;
            }
        }
    }
</style>
