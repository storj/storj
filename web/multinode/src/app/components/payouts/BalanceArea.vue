// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <info-block>
        <template #body>
            <div class="balance-area">
                <header class="balance-area__header">
                    <h3 class="balance-area__header__title">Balance</h3>
                </header>
                <div class="balance-area__content">
                    <div class="balance-area__content__item">
                        <p class="balance-area__content__item__label">Undistributed Payouts</p>
                        <p class="balance-area__content__item__value">{{ Currency.dollarsFromCents(undistributed) }}</p>
                    </div>
                    <div class="balance-area__content__item">
                        <p class="balance-area__content__item__label">Current Month Estimated Earnings</p>
                        <p class="balance-area__content__item__value">{{ Currency.dollarsFromCents(currentMonthEstimation) }}</p>
                    </div>
                </div>
            </div>
        </template>
    </info-block>
</template>

<script setup lang="ts">
import { Currency } from '@/app/utils/currency';

import InfoBlock from '@/app/components/common/InfoBlock.vue';

withDefaults(defineProps<{
    undistributed?: number;
    currentMonthEstimation?: number;
}>(), {
    undistributed: 0,
    currentMonthEstimation: 0,
});
</script>

<style scoped lang="scss">
    .balance-area {
        box-sizing: border-box;
        width: 100%;

        &__header {

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 24px;
                color: var(--c-header-base);
            }
        }

        &__content {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            margin-top: 34px;

            &__item {
                display: flex;
                align-items: flex-end;
                justify-content: space-between;
                width: 100%;

                &:not(:first-of-type) {
                    margin-top: 26px;
                }

                &__label {
                    font-size: 14px;
                    color: var(--v-text-base);
                    font-family: 'font_medium', sans-serif;
                }

                &__value {
                    font-size: 18px;
                    color: var(--v-header-base);
                    font-family: 'font_semiBold', sans-serif;
                }
            }
        }
    }
</style>
