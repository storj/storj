// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="section">
        <div class="section__wrapper">
            <component :is="iconAndTitle.icon" class="section__wrapper__icon" />
            <div class="section__wrapper__functional">
                <h2 class="section__wrapper__functional__title">{{ iconAndTitle.title }}</h2>
                <slot name="functional" />
            </div>
        </div>
        <slot v-if="slots.info" name="info" />
    </div>
</template>

<script setup lang="ts">
import { useSlots } from 'vue';

import { IconAndTitle } from '@/types/createAccessGrant';

const props = defineProps<{
    iconAndTitle: IconAndTitle;
}>();

const slots = useSlots();
</script>

<style scoped lang="scss">
.section {
    padding: 16px 0;
    border-bottom: 1px solid var(--c-grey-2);

    &__wrapper {
        display: flex;
        align-items: flex-start;

        &__icon {
            min-width: 40px;
        }

        &__functional {
            margin-left: 16px;
            width: calc(100% - 56px);

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-black);
                text-align: left;
                margin-bottom: 8px;
            }
        }
    }
}
</style>
