// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <info-block>
        <template #body>
            <div class="details-area">
                <header class="details-area__header">
                    <h3 class="details-area__header__title">Details</h3>
                    <p class="details-area__header__period">{{ period }}</p>
                </header>
                <div class="details-area__content">
                    <div class="details-area__content__item">
                        <p class="details-area__content__item__label">EARNED</p>
                        <p class="details-area__content__item__value">{{ Currency.dollarsFromCents(totalEarned) }}</p>
                    </div>
                    <div class="details-area__content__item">
                        <p class="details-area__content__item__label">HELD</p>
                        <p class="details-area__content__item__value">{{ Currency.dollarsFromCents(totalHeld) }}</p>
                    </div>
                    <div class="details-area__content__item">
                        <p class="details-area__content__item__label">PAID</p>
                        <p class="details-area__content__item__value">{{ Currency.dollarsFromCents(totalPaid) }}</p>
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
    totalEarned?: number;
    totalHeld?: number;
    totalPaid?: number;
    period?: string;
}>(), {
    totalEarned: 0,
    totalHeld: 0,
    totalPaid: 0,
    period: '',
});
</script>

<style scoped lang="scss">
    .details-area {
        box-sizing: border-box;
        width: 100%;

        &__header {
            display: flex;
            align-items: flex-end;
            justify-content: space-between;
            margin-bottom: 30px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 24px;
                color: var(--v-header-base);
            }

            &__period {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                color: var(--v-text-base);
            }
        }

        &__content {
            display: flex;
            align-items: flex-end;
            justify-content: space-between;

            &__item {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                font-family: 'font_semiBold', sans-serif;

                &:last-of-type {
                    align-items: flex-end;
                }

                &__label {
                    font-size: 12px;
                    color: var(--v-text-base);
                    margin-bottom: 10px;
                }

                &__value {
                    font-size: 18px;
                    color: var(--v-header-base);
                }
            }
        }
    }
</style>
