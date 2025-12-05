// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="held-history-container">
        <div class="held-history-container__header">
            <p class="held-history-container__header__title">Held Amount History</p>
            <div class="held-history-container__header__selection-area">
                <button
                    name="Select All Stats"
                    class="held-history-container__header__selection-area__item"
                    type="button"
                    :class="{ active: isAllStatsShown }"
                    @click="showAllStats"
                >
                    <p class="held-history-container__header__selection-area__item__label">
                        All Stats
                    </p>
                </button>
                <button
                    name="Select Monthly Breakdown"
                    class="held-history-container__header__selection-area__item"
                    type="button"
                    :class="{ active: !isAllStatsShown }"
                    @click="showMonthlyBreakdown"
                >
                    <p class="held-history-container__header__selection-area__item__label">
                        Monthly Breakdown
                    </p>
                </button>
            </div>
        </div>
        <div class="held-history-container__divider" />
        <HeldHistoryAllStatsTable v-if="isAllStatsShown" />
        <HeldHistoryMonthlyBreakdownTable v-else />
    </section>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import HeldHistoryAllStatsTable from '@/app/components/payments/HeldHistoryAllStatsTable.vue';
import HeldHistoryMonthlyBreakdownTable from '@/app/components/payments/HeldHistoryMonthlyBreakdownTable.vue';

const isAllStatsShown = ref(true);

function showAllStats(): void {
    isAllStatsShown.value = true;
}

function showMonthlyBreakdown(): void {
    isAllStatsShown.value = false;
}
</script>

<style scoped lang="scss">
    .held-history-container {
        display: flex;
        flex-direction: column;
        padding: 28px 40px 10px;
        background: var(--block-background-color);
        border: 1px solid var(--block-border-color);
        box-sizing: border-box;
        border-radius: 12px;
        margin: 12px 0 50px;

        &__header {
            display: flex;
            flex-direction: row;
            align-items: flex-start;
            justify-content: space-between;
            height: 40px;

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 18px;
                color: var(--regular-text-color);
            }

            &__selection-area {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                height: 100%;

                &__item {
                    display: flex;
                    align-items: flex-start;
                    justify-content: center;
                    cursor: pointer;
                    height: 100%;
                    padding: 0 20px;
                    border-bottom: 3px solid transparent;
                    z-index: 102;

                    &__label {
                        text-align: center;
                        font-size: 16px;
                        color: var(--regular-text-color);
                    }

                    &.active {
                        border-bottom: 3px solid var(--navigation-link-color);

                        &__label {
                            font-size: 16px;
                            color: var(--regular-text-color);
                        }
                    }
                }
            }
        }

        &__divider {
            width: 100%;
            height: 1px;
            background-color: #eaeaea;
        }
    }

    @media screen and (width <= 870px) {

        .held-history-container {

            &__divider {
                display: none;
            }

            &__header {
                flex-direction: column;
                align-items: flex-start;
                height: auto;

                &__selection-area {
                    width: 100%;
                    height: 41px;
                    margin: 20px 0;

                    &__item {
                        width: calc(50% - 40px);
                        border-bottom: 3px solid #eaeaea;
                    }
                }
            }
        }
    }

    @media screen and (width <= 600px) {

        .held-history-container {
            padding: 28px 20px 10px;
        }
    }
</style>
