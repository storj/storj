// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="range-selection">
        <div
            class="range-selection__toggle-container"
            :class="{ active: isOpen }"
            @click.stop="toggle"
        >
            <DatepickerIcon class="range-selection__toggle-container__icon" />
            <h1 class="range-selection__toggle-container__label">{{ dateRangeLabel }}</h1>
        </div>
        <div v-if="isOpen" v-click-outside="closePicker" class="range-selection__popup">
            <VDateRangePicker :on-date-pick="onDatePick" :is-open="true" :date-range="defaultDateRange" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { APP_STATE_ACTIONS } from "@/utils/constants/actionNames";
import { ProjectUsageDateRange } from "@/types/projects";

import VDateRangePicker from "@/components/common/VDateRangePicker.vue";

import DatepickerIcon from '@/../static/images/project/datepicker.svg';

// @vue/component
@Component({
    components: {
        DatepickerIcon,
        VDateRangePicker,
    },
})

export default class DateRangeSelection extends Vue {
    @Prop({ default: null })
    public readonly dateRange: ProjectUsageDateRange | null;
    @Prop({ default: () => false })
    public readonly onDatePick: (dateRange: Date[]) => void;
    @Prop({ default: () => false })
    public readonly toggle: () => void;
    @Prop({ default: false })
    public readonly isOpen: boolean;

    /**
     * Closes duration picker.
     */
    public closePicker(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Returns formatted date range string.
     */
    public get dateRangeLabel(): string {
        if (!this.dateRange) {
            return 'Last 30 days';
        }

        if (this.dateRange.since.getTime() === this.dateRange.before.getTime()) {
            return this.dateRange.since.toLocaleDateString('en-US')
        }

        const sinceFormattedString = this.dateRange.since.toLocaleDateString('en-US');
        const beforeFormattedString = this.dateRange.before.toLocaleDateString('en-US');
        return `${sinceFormattedString}-${beforeFormattedString}`;
    }

    /**
     * Returns default date range.
     */
    public get defaultDateRange(): Date[] {
        if (this.dateRange) {
            return [this.dateRange.since, this.dateRange.before]
        }

        const previous = new Date()
        previous.setMonth(previous.getMonth() - 1)
        return [previous, new Date()]
    }
}
</script>

<style scoped lang="scss">
    .range-selection {
        background-color: #fff;
        cursor: pointer;
        font-family: 'font_regular', sans-serif;
        position: relative;

        &__toggle-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 10px 16px;
            border-radius: 8px;
            border: 1px solid #d8dee3;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 13px;
                line-height: 20px;
                letter-spacing: -0.02em;
                color: #56606d;
                margin-left: 9px;
            }
        }

        &__popup {
            position: absolute;
            top: calc(100% + 5px);
            right: 0;
            width: 640px;
            box-shadow: 0 20px 34px rgba(10, 27, 44, 0.28);
            border-radius: 8px;
        }
    }

    .active {
        border-color: #0149ff;

        h1 {
            color: #0149ff;
        }

        svg path {
            fill: #0149ff;
        }
    }
</style>
