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
        <div v-show="isOpen" v-click-outside="closePicker" class="range-selection__popup">
            <VDateRangePicker :on-date-pick="onDatePick" :is-open="true" :date-range="pickerDateRange" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import VDateRangePicker from '@/components/common/VDateRangePicker.vue';

import DatepickerIcon from '@/../static/images/project/datepicker.svg';

// @vue/component
@Component({
    components: {
        DatepickerIcon,
        VDateRangePicker,
    },
})

export default class DateRangeSelection extends Vue {
    @Prop({ default: new Date() })
    public readonly since: Date;
    @Prop({ default: new Date() })
    public readonly before: Date;
    @Prop({ default: () => () => {} })
    public readonly onDatePick: (dateRange: Date[]) => void;
    @Prop({ default: () => () => {} })
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
        if (this.since.getTime() === this.before.getTime()) {
            return this.since.toLocaleDateString('en-US');
        }

        const sinceFormattedString = this.since.toLocaleDateString('en-US');
        const beforeFormattedString = this.before.toLocaleDateString('en-US');
        return `${sinceFormattedString}-${beforeFormattedString}`;
    }

    /**
     * Returns date range to be displayed in date range picker.
     */
    public get pickerDateRange(): Date[] {
        return [this.since, this.before];
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
            box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
            border-radius: 8px;
        }
    }

    .active {
        border-color: #0149ff;

        h1 {
            color: #0149ff;
        }

        svg :deep(path) {
            fill: #0149ff;
        }
    }
</style>
