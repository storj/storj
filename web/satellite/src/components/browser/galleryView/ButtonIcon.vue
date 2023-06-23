// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VInfo>
        <template #icon>
            <div class="button-icon" :class="{ active: isActive }" @click="onPress">
                <component :is="icon" />
            </div>
        </template>
        <template #message>
            <p class="message">{{ info }}</p>
        </template>
    </VInfo>
</template>

<script setup lang="ts">
import { Component } from 'vue';

import VInfo from '@/components/common/VInfo.vue';

const props = withDefaults(defineProps<{
    isActive?: boolean
    icon: string
    onPress: () => void
    info: string
}>(), {
    isActive: false,
});
</script>

<style scoped lang="scss">
.button-icon {
    width: 48px;
    height: 48px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;

    &:hover {
        background: rgb(255 255 255 / 10%);
    }
}

.active {
    background: rgb(255 255 255 / 10%);
}

.message {
    font-family: 'font_medium', sans-serif;
    font-size: 12px;
    line-height: 18px;
    color: var(--c-white);
    white-space: nowrap;
}

:deep(.info__box) {
    width: auto;
    cursor: default;
    top: 100%;
    left: 50%;
}

:deep(.info__box__message) {
    background: var(--c-grey-6);
    border-radius: 4px;
    padding: 10px 8px;
}

:deep(.info__box__arrow) {
    background: var(--c-grey-6);
    width: 10px;
    height: 10px;
    margin-bottom: -3px;
}
</style>
