// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wrapper">
        <component :is="iconAndTitle.icon" class="wrapper__icon" />
        <div class="wrapper__functional">
            <h2 class="wrapper__functional__title">{{ iconAndTitle.title }}</h2>
            <slot name="functional" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { IconAndTitle } from '@/types/createAccessGrant';

const props = defineProps<{
    iconAndTitle: IconAndTitle;
}>();
</script>

<style scoped lang="scss">
.wrapper {
    display: flex;
    align-items: flex-start;
    padding: 16px 0;
    border-bottom: 1px solid #ebeef1;

    &__icon {
        min-width: 40px;
    }

    &__functional {
        margin-left: 16px;
        width: 100%;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: #000;
            text-align: left;
            margin-bottom: 8px;
        }
    }
}
</style>
