// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <button
        name="Show Period Date Picker"
        class="period-container"
        type="button"
        :class="{ disabled: false }"
        @click.stop="openPeriodDropdown"
    >
        <p class="period-container__label long-text">Custom Date Range</p>
        <p class="period-container__label short-text">Custom Range</p>
        <BlackArrowHide v-if="isCalendarShown" />
        <BlackArrowExpand v-else />
        <PayoutPeriodCalendar
            v-if="isCalendarShown"
            v-click-outside="closePeriodDropdown"
            class="period-container__calendar"
        />
    </button>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useAppStore } from '@/app/store/modules/appStore';
import { useNodeStore } from '@/app/store/modules/nodeStore';

import PayoutPeriodCalendar from '@/app/components/payments/PayoutPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

const appStore = useAppStore();
const nodeStore = useNodeStore();

const isCalendarShown = computed<boolean>(() => {
    return appStore.state.isPayoutCalendarShown;
});

const isCalendarDisabled = computed<boolean>(() => {
    const nodeStartedAt = nodeStore.state.selectedSatellite.joinDate;
    const now = new Date();

    return nodeStartedAt.getUTCMonth() === now.getUTCMonth() && nodeStartedAt.getUTCFullYear() === now.getUTCFullYear();
});

function openPeriodDropdown(): void {
    if (isCalendarDisabled.value) {
        return;
    }

    appStore.togglePayoutCalendar(true);
}

function closePeriodDropdown(): void {
    appStore.togglePayoutCalendar(false);
}
</script>

<style scoped lang="scss">
    .period-container {
        position: relative;
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: center;
        background-color: transparent;
        cursor: pointer;

        &__label {
            margin-right: 8px;
            font-family: 'font_regular', sans-serif;
            font-weight: 500;
            font-size: 16px;
            color: var(--regular-text-color);
        }

        &__calendar {
            position: absolute;
            top: 30px;
            right: 0;
        }
    }

    .active {

        .period-container__label {
            color: var(--navigation-link-color);
        }
    }

    .arrow :deep(path) {
        fill: var(--period-selection-arrow-color);
    }

    .short-text {
        display: none;
    }

    .disabled {

        .period-container {

            &__label {
                color: #909bad;
            }
        }

        .arrow :deep(path) {
            fill: #909bad !important;
        }
    }

    @media screen and (width <= 505px) {

        .period-container__label {
            margin-right: 4px;
        }

        .short-text {
            display: inline-block;
            font-size: 14px;
        }

        .long-text {
            display: none;
        }
    }
</style>
