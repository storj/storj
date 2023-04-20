// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-click-outside="closePicker" class="date-picker">
        <ul class="date-picker__column">
            <li class="date-picker__column__item" tabindex="0" @click="onOneDayClick" @keyup.space="onOneDayClick">
                24 Hours
            </li>
            <li class="date-picker__column__item" tabindex="0" @click="onOneWeekClick" @keyup.space="onOneWeekClick">
                1 Week
            </li>
            <li class="date-picker__column__item" tabindex="0" @click="onOneMonthClick" @keyup.space="onOneMonthClick">
                1 month
            </li>
            <li class="date-picker__column__item" tabindex="0" @click="onSixMonthsClick" @keyup.space="onSixMonthsClick">
                6 Months
            </li>
            <li class="date-picker__column__item" tabindex="0" @click="onOneYearClick" @keyup.space="onOneYearClick">
                1 Year
            </li>
            <li class="date-picker__column__item" tabindex="0" @click="onForeverClick" @keyup.space="onForeverClick">
                No end date
            </li>
        </ul>
        <VDatePicker :on-date-pick="onCustomDatePick" />
    </div>
</template>

<script setup lang="ts">
import { useAppStore } from '@/store/modules/appStore';

import VDatePicker from '@/components/common/VDatePicker.vue';

const props = defineProps<{
    setLabel: (label: string) => void;
    setNotAfter: (date: Date | undefined) => void;
}>();

const appStore = useAppStore();

/**
 * Closes date picker.
 */
function closePicker(): void {
    appStore.closeDropdowns();
}

/**
 * onCustomDatePick holds logic for choosing custom date.
 * @param date
 */
function onCustomDatePick(date: Date): void {
    const to = new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59);
    const toFormattedString = to.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    props.setLabel(toFormattedString);
    props.setNotAfter(to);
    closePicker();
}

/**
 * Holds on "No end date" choice click logic.
 */
function onForeverClick(): void {
    props.setLabel('No end date');
    props.setNotAfter(undefined);
    closePicker();
}

/**
 * Holds on "1 month" choice click logic.
 */
function onOneMonthClick(): void {
    const now = new Date();
    const inAMonth = new Date(now.setMonth(now.getMonth() + 1));

    props.setLabel('1 Month');
    props.setNotAfter(inAMonth);
    closePicker();
}

/**
 * Holds on "24 hours" choice click logic.
 */
function onOneDayClick(): void {
    const now = new Date();
    const inADay = new Date(now.setDate(now.getDate() + 1));

    props.setLabel('24 Hours');
    props.setNotAfter(inADay);
    closePicker();
}

/**
 * Holds on "1 week" choice click logic.
 */
function onOneWeekClick(): void {
    const now = new Date();
    const inAWeek = new Date(now.setDate(now.getDate() + 7));

    props.setLabel('1 Week');
    props.setNotAfter(inAWeek);
    closePicker();
}

/**
 * Holds on "6 month" choice click logic.
 */
function onSixMonthsClick(): void {
    const now = new Date();
    const inSixMonth = new Date(now.setMonth(now.getMonth() + 6));

    props.setLabel('6 Months');
    props.setNotAfter(inSixMonth);
    closePicker();
}

/**
 * Holds on "1 year" choice click logic.
 */
function onOneYearClick(): void {
    const now = new Date();
    const inOneYear = new Date(now.setFullYear(now.getFullYear() + 1));

    props.setLabel('1 Year');
    props.setNotAfter(inOneYear);
    closePicker();
}
</script>

<style scoped lang="scss">
    .date-picker {
        background: var(--c-white);
        width: 410px;
        border: 1px solid var(--c-grey-7);
        border-radius: 6px;
        box-shadow: 0 4px 8px 0 rgb(0 0 0 / 20%), 0 6px 20px 0 rgb(0 0 0 / 19%);
        position: absolute;
        z-index: 1;
        top: 100%;
        left: 0;
        display: flex;
        align-items: center;
        cursor: default;

        @media screen and (max-width: 600px) {
            left: -90px;
        }

        @media screen and (max-width: 460px) {
            flex-direction: column;
            width: 320px;
            left: -78px;
        }

        &__column {
            list-style-type: none;
            padding-left: 0;
            margin-top: 0;

            @media screen and (max-width: 460px) {
                columns: 2;
                width: 100%;
            }

            &__item {
                font-size: 14px;
                font-weight: 400;
                padding: 10px 12px;
                color: var(--c-grey-8);
                cursor: pointer;
                white-space: nowrap;

                &:hover {
                    font-weight: bold;
                    background: var(--c-grey-0);
                }
            }
        }
    }
</style>
