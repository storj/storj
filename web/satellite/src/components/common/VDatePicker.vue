// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <DatePicker
        aria-roledescription="datepicker"
        :open="true"
        :inline="true"
        :value="date"
        popup-class="picker"
        @change="onDatePick"
    />
</template>

<script setup lang="ts">
import DatePicker from 'vue2-datepicker';

const props = withDefaults(defineProps<{
    onDatePick?: (date: Date) => void;
    date?: Date;
}>(), {
    onDatePick: () => () => false,
    date: () => new Date(),
});
</script>

<style scoped lang="scss">
    @import '~vue2-datepicker/scss/index';

    .picker,
    .mx-datepicker {
        width: 100%;
        cursor: default;
    }

    .mx-calendar {
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
        color: #000;
    }

    .mx-calendar-content .cell.in-range {
        color: #000;
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
        color: #000;
    }

    .mx-btn:hover {
        border-color: #000;
        color: #000;
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

    @media screen and (max-width: 768px) {

        .mx-range-wrapper {
            flex-direction: column;
        }

        .mx-calendar + .mx-calendar {
            border-left: none;
            border-top: 1px solid #e8e8e8;
        }
    }
</style>
