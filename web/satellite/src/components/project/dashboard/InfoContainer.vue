// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="info-container">
        <h2 class="info-container__title">{{ title }}</h2>
        <VLoader v-if="isDataFetching" height="40px" width="40px" />
        <template v-else>
            <p class="info-container__subtitle">{{ subtitle }}</p>
            <p class="info-container__value" aria-roledescription="info-value">{{ value }}</p>
            <slot name="side-value" />
        </template>
    </div>
</template>

<script setup lang="ts">
import VLoader from '@/components/common/VLoader.vue';

const props = withDefaults(defineProps<{
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
        background-color: #fff;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 10px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 18px;
            line-height: 27px;
            color: #000;
        }

        &__subtitle {
            font-size: 12px;
            line-height: 18px;
            color: #000;
            margin-bottom: 24px;
        }

        &__value {
            font-family: 'font_bold', sans-serif;
            font-size: 36px;
            line-height: 47px;
            letter-spacing: -0.02em;
            color: #000;
        }
    }
</style>
