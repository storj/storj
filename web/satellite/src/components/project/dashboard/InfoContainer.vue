// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-container">
        <div class="info-container__header">
            <component :is="icon" />
            <h2 class="info-container__header__title">{{ title }}</h2>
        </div>
        <VLoader v-if="isDataFetching" height="40px" width="40px" />
        <template v-else>
            <p class="info-container__subtitle">{{ subtitle }}</p>
            <p class="info-container__value" aria-roledescription="info-value">{{ value }}</p>
            <slot name="side-value" />
        </template>
    </div>
</template>

<script setup lang="ts">
import { Component } from 'vue';

import VLoader from '@/components/common/VLoader.vue';

const props = withDefaults(defineProps<{
    icon: Component,
    isDataFetching: boolean,
    title: string,
    subtitle: string,
    value: string,
}>(), {
    isDataFetching: false,
    title: '',
    subtitle: '',
    value: '',
});
</script>

<style scoped lang="scss">
    .info-container {
        padding: 24px;
        width: calc(100% - 48px);
        font-family: 'font_regular', sans-serif;
        background-color: var(--c-white);
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 10px;

        &__header {
            display: flex;
            align-items: center;

            :deep(path) {
                fill: var(--c-black);
            }

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 18px;
                line-height: 27px;
                color: var(--c-black);
                margin-left: 8px;
            }
        }

        &__subtitle {
            font-size: 12px;
            line-height: 18px;
            color: var(--c-grey-6);
            margin-bottom: 24px;
        }

        &__value {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: var(--c-black);
        }
    }
</style>
