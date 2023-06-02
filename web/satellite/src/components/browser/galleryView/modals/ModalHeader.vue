// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <component :is="icon" />
        <h1 class="header__title">{{ title }}</h1>
    </div>
</template>

<script setup lang="ts">
const props = defineProps<{
    icon: string
    title: string
}>();
</script>

<style scoped lang="scss">
.header {
    display: flex;
    align-items: center;
    padding-bottom: 16px;
    margin-bottom: 16px;
    border-bottom: 1px solid var(--c-grey-2);

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        letter-spacing: -0.02em;
        color: var(--c-black);
        margin-left: 16px;
    }
}
</style>
