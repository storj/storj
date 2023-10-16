// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="range-selection">
        <div
            class="range-selection__toggle-container"
            :class="{ active: isOpen }"
            aria-roledescription="datepicker-toggle"
            @click.stop="toggle"
        >
            <DatepickerIcon class="range-selection__toggle-container__icon" />
            <h1 class="range-selection__toggle-container__label">{{ dateRangeLabel }}</h1>
        </div>
        <div v-if="isOpen" v-click-outside="closePicker" class="range-selection__popup">
            <VDateRangePicker :on-date-pick="onDatePick" :is-open="true" :date-range="pickerDateRange" />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { useAppStore } from '@/store/modules/appStore';

import VDateRangePicker from '@/components/common/VDateRangePicker.vue';

import DatepickerIcon from '@/../static/images/project/datepicker.svg';

const props = withDefaults(defineProps<{
    since: Date,
    before: Date,
    isOpen: boolean,
    onDatePick: (dateRange: Date[]) => void,
    toggle: () => void;
}>(), {
    since: () => new Date(),
    before: () => new Date(),
    isOpen: false,
    onDatePick: () => {},
    toggle: () => {},
});

const appStore = useAppStore();

/**
 * Returns formatted date range string.
 */
const dateRangeLabel = computed((): string => {
    if (props.since.getTime() === props.before.getTime()) {
        return props.since.toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
    }

    const sinceFormattedString = props.since.toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
    const beforeFormattedString = props.before.toLocaleDateString('en-US', { day:'numeric', month:'short', year:'numeric' });
    return `${sinceFormattedString} - ${beforeFormattedString}`;
});

/**
 * Returns date range to be displayed in date range picker.
 */
const pickerDateRange = computed((): Date[] => {
    return [props.since, props.before];
});

/**
 * Closes duration picker.
 */
function closePicker(): void {
    appStore.closeDropdowns();
}
</script>

<style scoped lang="scss">
    .range-selection {
        background-color: var(--c-white);
        cursor: pointer;
        font-family: 'font_regular', sans-serif;
        position: relative;
        border-radius: 8px;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px 16px;
            border-radius: 8px;
            border: 1px solid var(--c-grey-3);

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 13px;
                line-height: 20px;
                letter-spacing: -0.02em;
                color: var(--c-grey-6);
                margin-left: 9px;
            }
        }

        &__popup {
            position: absolute;
            top: calc(100% + 5px);
            right: 0;
            width: 640px;
            box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
            border-radius: 8px;
        }
    }

    .active {
        border-color: var(--c-blue-3);

        h1 {
            color: var(--c-blue-3);
        }

        svg :deep(path) {
            fill: var(--c-blue-3);
        }
    }
</style>
