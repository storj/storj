// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="duration-picker">
        <div class="duration-picker__list">
            <ul class="duration-picker__list__column">
                <li @click="onForeverClick" class="duration-picker__list__column-item">Forever</li>
                <li @click="onOneDayClick" class="duration-picker__list__column-item">24 Hours</li>
                <li @click="onOneWeekClick" class="duration-picker__list__column-item">1 Week</li>
            </ul>
            <ul class="duration-picker__list__column">
                <li @click="onOneMonthClick" class="duration-picker__list__column-item">1 month</li>
                <li @click="onSixMonthsClick" class="duration-picker__list__column-item">6 Months</li>
                <li @click="onOneYearClick" class="duration-picker__list__column-item">1 Year</li>
            </ul>
        </div>
        <hr class="duration-picker__break">
        <div class="duration-picker__date-picker__wrapper">
            <DatePicker
                range
                open
                :append-to-body="false"
                :inline="true"
                popup-class="duration-picker__date-picker__popup"
                @change="onCustomRangePick"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DatePicker from 'vue2-datepicker';
import 'vue2-datepicker/index.css';

import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { DurationPermission } from '@/types/accessGrants';

@Component({
    components: {
        DatePicker,
    },
})
export default class DurationPicker extends Vue {
    /**
     * onCustomRangePick holds logic for choosing custom date range.
     * @param dateRange
     */
    public onCustomRangePick(dateRange: Date[]): void {
        const permission: DurationPermission = new DurationPermission(dateRange[0], dateRange[1]);
        const fromFormattedString = dateRange[0].toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
        const toFormattedString = dateRange[1].toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
        const rangeLabel = `${fromFormattedString} - ${toFormattedString}`;

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', rangeLabel);
    }

    /**
     * Holds on "forever" choice click logic.
     */
    public onForeverClick(): void {
        const permission = new DurationPermission(new Date(), new Date('2200-01-01'));

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', 'Forever');
        this.$emit('close');
    }

    /**
     * Holds on "1 month" choice click logic.
     */
    public onOneMonthClick(): void {
        const now = new Date();
        const inAMonth = new Date(now.setMonth(now.getMonth() + 1));
        const permission = new DurationPermission(new Date(), inAMonth);

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', '1 Month');
        this.$emit('close');
    }

    /**
     * Holds on "24 hours" choice click logic.
     */
    public onOneDayClick(): void {
        const now = new Date();
        const inADay = new Date(now.setDate(now.getDate() + 1));
        const permission = new DurationPermission(new Date(), inADay);

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', '24 Hours');
        this.$emit('close');
    }

    /**
     * Holds on "1 week" choice click logic.
     */
    public onOneWeekClick(): void {
        const now = new Date();
        const inAWeek = new Date(now.setDate(now.getDate() + 7));
        const permission = new DurationPermission(new Date(), inAWeek);

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', '1 Week');
        this.$emit('close');
    }

    /**
     * Holds on "6 month" choice click logic.
     */
    public onSixMonthsClick(): void {
        const now = new Date();
        const inSixMonth = new Date(now.setMonth(now.getMonth() + 6));
        const permission = new DurationPermission(new Date(), inSixMonth);

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', '6 Months');
        this.$emit('close');
    }

    /**
     * Holds on "1 year" choice click logic.
     */
    public onOneYearClick(): void {
        const now = new Date();
        const inOneYear = new Date(now.setFullYear(now.getFullYear() + 1));
        const permission = new DurationPermission(new Date(), inOneYear);

        this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_DURATION_PERMISSION, permission);
        this.$emit('setLabel', '1 Year');
        this.$emit('close');
    }
}
</script>

<style scoped lang="scss">
    .duration-picker {
        background: #fff;
        width: 530px;
        height: 400px;
        border: 1px solid #384b65;
        margin: 0 auto;
        -webkit-box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        -moz-box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        box-shadow: 0 4px 8px 0 rgba(0, 0, 0, 0.2), 0 6px 20px 0 rgba(0, 0, 0, 0.19);
        position: absolute;
        z-index: 1;
        right: 0;
        top: 100%;

        &__list {
            -moz-column-count: 2;
            -moz-column-gap: 48px;
            -webkit-column-count: 2;
            column-count: 2;
            padding: 10px 24px 0;

            &__column {
                list-style-type: none;
                padding-left: 0;
                margin-top: 0;
            }

            &__column-item {
                font-size: 14px;
                font-weight: 400;
                padding: 10px 0 10px 12px;
                border-left: 7px solid #fff;
                color: #1b2533;

                &:hover {
                    font-weight: bold;
                    background: #f5f6fa;
                    border-left: 6px solid #2582ff;
                    cursor: pointer;
                }
            }
        }

        &__break {
            width: 84%;
            margin: 0 auto;
        }

        &__date-picker {

            &__wrapper {
                position: relative;
                margin: 0;
            }
        }
    }
</style>

<style lang="scss">
    .duration-picker {

        &__date-picker {

            &__popup {
                position: absolute;
                top: 0;
                left: 25px;
                width: 480px;
                height: 210px;
            }
        }
    }

    .mx-calendar {
        height: 210px;
    }

    .mx-table-date td,
    .mx-table-date th {
        height: 12px;
        font-size: 10px;
    }

    .mx-table {
        height: 70%;
    }
</style>
