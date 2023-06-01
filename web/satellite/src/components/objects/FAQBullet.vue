// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bullet">
        <div class="bullet__header" @click="toggle">
            <h2 class="bullet__header__title">{{ title }}</h2>
            <MinusIcon v-if="isExpanded" />
            <PlusIcon v-else />
        </div>
        <p v-if="isExpanded" class="bullet__text">{{ text }}</p>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import PlusIcon from '@/../static/images/common/plus.svg';
import MinusIcon from '@/../static/images/common/minus.svg';

const props = withDefaults(defineProps<{
    title: string;
    text: string;
}>(), {
    title: '',
    text: '',
});

const isExpanded = ref<boolean>(false);

/**
 * Toggles bullet text visibility.
 */
function toggle(): void {
    isExpanded.value = !isExpanded.value;
}
</script>

<style scoped lang="scss">
    .bullet {
        background: #fff;
        border: 2px solid var(--c-grey-3);
        border-radius: 16px;
        padding: 28px 32px;
        width: calc(100% - 64px);
        margin-bottom: 30px;

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            cursor: pointer;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 32px;
                letter-spacing: 0.75px;
                color: var(--c-blue-6);
            }
        }

        &__text {
            margin-top: 12px;
            font-size: 16px;
            line-height: 32px;
            color: var(--c-grey-6);
        }
    }
</style>
