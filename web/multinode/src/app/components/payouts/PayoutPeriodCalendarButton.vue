// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        class="calendar-button"
        @click.stop="toggleCalendar"
    >
        <span class="label">{{ period }}</span>
        <svg width="8" height="4" viewBox="0 0 8 4" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M3.33657 3.73107C3.70296 4.09114 4.29941 4.08814 4.66237 3.73107L7.79796 0.650836C8.16435 0.291517 8.01864 0 7.47247 0L0.526407 0C-0.0197628 0 -0.16292 0.294525 0.200917 0.650836L3.33657 3.73107Z" fill="currentColor" />
        </svg>
        <payout-period-calendar v-if="isCalendarShown" class="calendar-button__calendar" @on-close="closeCalendar" />
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import PayoutPeriodCalendar from './PayoutPeriodCalendar.vue';

withDefaults(defineProps<{
    period?: string;
}>(), {
    period: '',
});

const isCalendarShown = ref<boolean>(false);

function toggleCalendar(): void {
    isCalendarShown.value = !isCalendarShown.value;
}

function closeCalendar(): void {
    if (!isCalendarShown.value) return;

    setTimeout(() => {
        isCalendarShown.value = false;
    }, 1);
}
</script>

<style lang="scss">
    .calendar-button {
        position: relative;
        box-sizing: border-box;
        width: 100%;
        max-width: 300px;
        height: 40px;
        background: transparent;
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0 16px;
        border: 1px solid var(--v-border-base);
        border-radius: 6px;
        font-size: 16px;
        color: var(--v-text-base);
        cursor: pointer;
        font-family: 'font_medium', sans-serif;

        &:hover {
            border-color: var(--c-gray);
        }

        &__calendar {
            position: absolute;
            top: 50px;
            right: 0;
        }
    }

    .label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
</style>
