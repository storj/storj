// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VueDatePicker
        aria-roledescription="datepicker"
        range
        :open="isOpen"
        :value="dateRange"
        :editable="false"
        :clearable="false"
        :append-to-body="false"
        popup-class="picker"
        :popup-style="{position: 'relative', left: 0, top: 0}"
        @change="onDatePick"
    />
</template>

<script setup lang="ts">
import VueDatePicker from 'vue-datepicker-next';

const props = withDefaults(defineProps<{
    isOpen?: boolean;
    onDatePick?: (dateRange: Date[]) => void;
    dateRange?: Date[];
}>(), {
    isOpen: false,
    onDatePick: () => () => false,
    dateRange: () => [],
});
</script>

<style lang="scss">
    @import 'vue-datepicker-next/index.css';

    .mx-input-wrapper {
        display: none;
    }

    .mx-datepicker-popup {
        box-shadow: unset;
    }

    .mx-calendar {
        width: 100%;
    }

    .picker,
    .mx-datepicker,
    .mx-range-wrapper {
        border: none;
        border-radius: 8px;
        cursor: default;
        width: 100%;
    }

    .mx-date-row {
        height: 40px;
    }

    .mx-table th {
        color: var(--c-grey-4);
    }

    .mx-table-date .cell.not-current-month {
        color: var(--c-grey-5);
    }

    .mx-calendar-content .cell {
        font-size: 12px;
        line-height: 18px;
        color: var(--c-black);
    }

    .mx-calendar-content .cell.in-range {
        color: var(--c-black);
        background-color: var(--c-blue-1);
        border-radius: 999px;
    }

    .mx-calendar-content .cell.active {
        background-color: var(--c-blue-3);
        border-radius: 999px;
    }

    .mx-calendar-content {
        height: unset;
    }

    .mx-btn-current-month,
    .mx-btn-current-year {
        pointer-events: none;
        font-family: 'font_medium', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
    }

    .mx-btn:hover {
        border-color: var(--c-black);
        color: var(--c-black);
    }

    .mx-btn-icon-double-right,
    .mx-btn-icon-double-left {
        display: none;
    }

    .mx-icon-left:before,
    .mx-icon-right:before {
        width: 20px;
        height: 20px;
        border-width: 4px 0 0 4px;
    }

    @media screen and (width <= 768px) {

        .range-selection__popup {
            width: 320px !important;
        }

        .mx-range-wrapper {
            flex-direction: column;
        }

        .mx-calendar + .mx-calendar {
            border-left: none;
            border-top: 1px solid #e8e8e8;
        }
    }
</style>
