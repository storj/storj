// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <DatePicker
        aria-roledescription="datepicker"
        range
        :open="isOpen"
        :inline="true"
        :value="dateRange"
        popup-class="picker"
        @change="onDatePick"
    />
</template>

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import DatePicker from 'vue2-datepicker';
import 'vue2-datepicker/index.css';

// @vue/component
@Component({
    components: {
        DatePicker,
    },
})
export default class VDateRangePicker extends Vue {
    @Prop({ default: false })
    public readonly isOpen: boolean
    @Prop({ default: () => false })
    public readonly onDatePick: (dateRange: Date[]) => void
    @Prop({ default: undefined })
    public readonly dateRange: Date[]
}
</script>

<style lang="scss">
    .picker,
    .mx-datepicker,
    .mx-range-wrapper {
        width: 100%;
        border: none;
        border-radius: 8px;
        cursor: default;
    }

    .mx-calendar {
        width: 50%;
    }

    .mx-date-row {
        height: 40px;
    }

    .mx-table th {
        color: #c8d3de;
    }

    .mx-table-date .cell.not-current-month {
        color: #929fb1;
    }

    .mx-calendar-content .cell {
        font-size: 12px;
        line-height: 18px;
        color: #000;
    }

    .mx-calendar-content .cell.in-range {
        color: #000;
        background-color: #e6edf7;
        border-radius: 999px;
    }

    .mx-calendar-content .cell.active {
        background-color: #0149ff;
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
</style>
