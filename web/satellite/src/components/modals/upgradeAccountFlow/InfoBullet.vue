// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bullet">
        <GreyCheckmark v-if="!isPro" class="bullet__icon" />
        <GreenCheckmark v-else class="bullet__icon" />
        <div class="bullet__column">
            <div class="bullet__column__header">
                <h3 class="bullet__column__header__title">{{ title }}</h3>
                <VInfo v-if="slots.moreInfo">
                    <template #icon>
                        <InfoIcon class="bullet__column__header__icon" />
                    </template>
                    <template #message>
                        <slot name="moreInfo" />
                    </template>
                </VInfo>
            </div>
            <p class="bullet__column__info">{{ info }}</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { useSlots } from 'vue';

import VInfo from '@/components/common/VInfo.vue';

import GreyCheckmark from '@/../static/images/modals/upgradeFlow/greyCheckmark.svg';
import GreenCheckmark from '@/../static/images/modals/upgradeFlow/greenCheckmark.svg';
import InfoIcon from '@/../static/images/modals/upgradeFlow/info.svg';

const props = withDefaults(defineProps<{
    isPro?: boolean;
    title: string;
    info: string;
}>(), {
    isPro: false,
    title: '',
    info: '',
});

const slots = useSlots();
</script>

<style scoped lang="scss">
.bullet {
    display: flex;
    align-items: flex-start;
    font-family: 'font_regular', sans-serif;

    &__icon {
        min-width: 16px;
        margin-top: 2px;
    }

    &__column {
        margin-left: 10px;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);

        &__header {
            display: flex;
            align-items: center;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-black);
                white-space: nowrap;
                margin-right: 6px;
            }

            &__icon {
                cursor: pointer;
                max-height: 14px;
            }
        }

        &__info {
            text-align: left;
        }
    }
}

:deep(.info) {
    max-height: 14px;
}

:deep(.info__box) {
    top: calc(100% + 1px);
    cursor: default;
    filter: none;
}

:deep(.info__box__message) {
    width: 245px;
    background: var(--c-grey-6);
    border-radius: 4px;
    padding: 10px 8px;
}

:deep(.info__box__arrow) {
    background: var(--c-grey-6);
    width: 10px;
    height: 10px;
    margin-bottom: -3px;
    border-radius: 0;
}
</style>
