// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="blured-container">
        <p v-if="isMnemonic" class="blured-container__mnemonic">{{ value }}</p>
        <p v-else class="blured-container__text">{{ value }}</p>
        <CopyIcon v-if="!isMnemonic" class="blured-container__copy" />
        <div v-if="!isValueShown" class="blured-container__blur">
            <VButton
                :label="buttonLabel"
                icon="lock"
                width="159px"
                height="40px"
                font-size="12px"
                :is-white="true"
                :on-press="showValue"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import VButton from '@/components/common/VButton.vue';

import CopyIcon from '@/../static/images/accessGrants/newCreateFlow/copy.svg';

const props = defineProps<{
    isMnemonic: boolean;
    value: string;
    buttonLabel: string;
}>();

const isValueShown = ref<boolean>(false);

/**
 * Makes blurred value to be shown.
 */
function showValue(): void {
    isValueShown.value = true;
}
</script>

<style scoped lang="scss">
.blured-container {
    display: flex;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    padding: 10px 16px;
    background: var(--c-grey-2);
    border: 1px solid var(--c-grey-3);
    border-radius: 10px;
    position: relative;

    &__mnemonic {
        font-size: 14px;
        line-height: 26px;
        color: var(--c-black);
        text-align: justify-all;
    }

    &__text {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-grey-7);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        margin-right: 16px;
    }

    &__copy {
        min-width: 16px;
        cursor: pointer;
    }

    &__blur {
        position: absolute;
        left: 0;
        right: 0;
        top: 0;
        bottom: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 10px;
        backdrop-filter: blur(10px);
    }
}
</style>
