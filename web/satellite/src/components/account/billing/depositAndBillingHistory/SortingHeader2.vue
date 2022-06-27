// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item date"
            @click="sortDate"
        >
            <p class="sort-header-container__item__name">DATE</p>
            <VerticalArrows2
                :is-active="arrowController.date"
                :direction="dateDirection"
            />
        </div>
        <div class="sort-header-container__item transaction">
            <p class="sort-header-container__item__name">TRANSACTION</p>
        </div>
        <div class="sort-header-container__item amount"
            @click="sortAmount"
        >
            <p class="sort-header-container__item__name">AMOUNT(USD)</p>
            <VerticalArrows2
                :is-active="arrowController.amount"
                :direction="amountDirection"
            />
        </div>
        <div class="sort-header-container__item status"
            @click="sortStatus"
        >
            <p class="sort-header-container__item__name">STATUS</p>
            <VerticalArrows2
                :is-active="arrowController.status"
                :direction="statusDirection"
            />
        </div>
        <div class="sort-header-container__item details">
            <p class="sort-header-container__item__name">DETAILS</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { SortDirection } from '@/types/common';

import VerticalArrows2 from '@/components/common/VerticalArrows2.vue';


// @vue/component
@Component({
    components: {
        VerticalArrows2,
    },
})
export default class SortingHeader2 extends Vue {

    public dateDirection: string = 'date-descending'
    public amountDirection: string = 'amount-descending'
    public statusDirection: string = 'status-descending'

    // public arePaymentsSortedByDate: boolean = false;
    // public arePaymentsSortedByAmount: boolean = false;
    // public arePaymentsSortedByStatus: boolean = false;

    public arrowController: {date: boolean, amount: boolean, status: boolean} = {date: false, amount: false, status: false}


    // sorts table by date
    public sortDate(): void {
        this.$emit('sortFunction', this.dateDirection);
        this.dateDirection === 'date-ascending'? this.dateDirection = 'date-descending' : this.dateDirection = 'date-ascending';
        this.arrowController = {date: true, amount: false, status: false};
    }

    // sorts table by amount
    public sortAmount(): void {
        this.$emit('sortFunction', this.amountDirection);
        this.amountDirection === 'amount-ascending'? this.amountDirection = 'amount-descending' : this.amountDirection = 'amount-ascending';
        this.arrowController = {date: false, amount: true, status: false};
    }

    // sorts table by status
    public sortStatus(): void {
        this.$emit('sortFunction', this.statusDirection);
        this.statusDirection === 'status-ascending'? this.statusDirection = 'status-descending' : this.statusDirection = 'status-ascending';
        this.arrowController = {date: false, amount: false, status: true};
    }
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        width: 100%;
        padding: 16px 0;

        &__item {
            text-align: left;

            &__name {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 19px;
                color: #adadad;
                margin: 0;
            }
        }
    }

    .date, .amount, .status {
        display: flex;
        cursor: pointer;
    }

    .date {
        width: 15%;
    }

    .transaction {
        width: 35%;
    }

    .status {
        width: 15%;
    }

    .amount {
        width: 15%;
        margin: 0;
    }

    .details {
        text-align: left;
        margin: 0;
        width: 20%;
    }
</style>
