// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="upgrade-wrapper">
        <div class="upgrade-wrapper__header">
            <component :is="icon" v-if="icon" class="upgrade-wrapper__header__icon " />
            <h1 class="upgrade-wrapper__header__title">{{ title }}</h1>
        </div>
        <slot name="content" />
    </div>
</template>

<script setup lang="ts">
import Vue, { VueConstructor } from 'vue';

const props = defineProps<{
    icon?: VueConstructor<Vue>;
    title: string;
}>();
</script>

<style scoped lang="scss">
.upgrade-wrapper {
    padding: 32px;
    font-family: 'font_bold', sans-serif;

    @media screen and (max-width: 350px) {
        padding: 32px 16px;
    }

    &__header {
        display: flex;
        align-items: center;
        padding-bottom: 16px;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);

        @media screen and (max-width: 690px) {
            display: none;
        }

        &__icon {
            margin-right: 16px;
        }

        &__title {
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: var(--c-black);
            text-align: left;
        }
    }
}
</style>
