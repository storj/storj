// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="period-container" @click.stop="openPeriodDropdown">
        <p class="period-container__label">{{ currentPeriod }}</p>
        <BlackArrowHide v-if="isDropDownShown" />
        <BlackArrowExpand v-else />
        <PayoutPeriodCalendar
            class="period-container__calendar"
            v-if="isDropDownShown"
            v-click-outside="closePeriodDropdown"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import PayoutPeriodCalendar from '@/app/components/payments/PayoutPeriodCalendar.vue';

import BlackArrowExpand from '@/../static/images/BlackArrowExpand.svg';
import BlackArrowHide from '@/../static/images/BlackArrowHide.svg';

@Component({
    components: {
        PayoutPeriodCalendar,
        BlackArrowExpand,
        BlackArrowHide,
    }
})
export default class EstimationPeriodDropdown extends Vue {
    public currentPeriod = new Date().toDateString();
    /**
     * Indicates if payout period selection dropdown should be rendered.
     */
    public isDropDownShown: boolean = false;

    /**
     * Opens payout period selection dropdown.
     */
    public openPeriodDropdown(): void {
        setTimeout(() => {
            this.isDropDownShown = true;
        }, 0);
    }

    /**
     * Closes payout period selection dropdown.
     */
    public closePeriodDropdown(): void {
        this.isDropDownShown = false;
    }
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
            color: #535f77;
        }

        &__calendar {
            position: absolute;
            top: 30px;
            right: 0;
        }
    }
</style>
