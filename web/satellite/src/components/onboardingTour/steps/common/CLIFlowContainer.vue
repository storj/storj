// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="flow-container">
        <slot name="icon" />
        <h1 class="flow-container__title" aria-roledescription="title">{{ title }}</h1>
        <slot name="content" />
        <div class="flow-container__buttons">
            <VButton
                label="Back"
                height="48px"
                :is-white="true"
                :on-press="onBackClick"
                :is-disabled="isLoading"
            />
            <VButton
                label="Continue ->"
                height="48px"
                :on-press="onNextClick"
                :is-disabled="isLoading"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    onNextClick: () => unknown;
    onBackClick: () => unknown;
    title: string;
    isLoading?: boolean;
}>(), {
    onNextClick: () => {},
    onBackClick: () => {},
    title: '',
    isLoading: false,
});
</script>

<style scoped lang="scss">
    .flow-container {
        font-family: 'font_regular', sans-serif;
        background: #fff;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-radius: 20px;
        padding: 48px;
        max-width: 484px;
        display: flex;
        flex-direction: column;
        align-items: center;

        @media screen and (width <= 600px) {
            padding: 24px;
        }

        &__title {
            margin: 20px 0;
            font-family: 'font_Bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            text-align: center;
            color: #14142b;
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 34px;
            column-gap: 24px;

            @media screen and (width <= 450px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 24px;
            }
        }
    }
</style>
