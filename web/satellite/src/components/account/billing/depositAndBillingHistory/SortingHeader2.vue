// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div
            class="sort-header-container__item date"
            @click="sortFunction('date')"
        >
            <p class="sort-header-container__item__name">DATE</p>
            <VerticalArrows
                :is-active="arrowController.date"
                :direction="dateSortDirection"
            />
        </div>
        <div class="sort-header-container__item transaction">
            <p class="sort-header-container__item__name">TRANSACTION</p>
        </div>
        <div
            class="sort-header-container__item amount"
            @click="sortFunction('amount')"
        >
            <p class="sort-header-container__item__name">AMOUNT(USD)</p>
            <VerticalArrows
                :is-active="arrowController.amount"
                :direction="amountSortDirection"
            />
        </div>
        <div
            class="sort-header-container__item status"
            @click="sortFunction('status')"
        >
            <p class="sort-header-container__item__name">STATUS</p>
            <VerticalArrows
                :is-active="arrowController.status"
                :direction="statusSortDirection"
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

import VerticalArrows from '@/components/common/VerticalArrows.vue';

// @vue/component
@Component({
    components: {
        VerticalArrows,
    },
})
export default class SortingHeader2 extends Vue {

    public DATE_DIRECTION = 'date-descending';
    public AMOUNT_DIRECTION = 'amount-descending';
    public STATUS_DIRECTION = 'status-descending';

    public dateSortDirection: SortDirection = SortDirection.ASCENDING;
    public amountSortDirection: SortDirection = SortDirection.ASCENDING;
    public statusSortDirection: SortDirection = SortDirection.ASCENDING;

    public arrowController: {date: boolean, amount: boolean, status: boolean} = { date: false, amount: false, status: false };

    /**
     * sorts table by date
     */ 
    public sortFunction(key): void {
        switch (key) {
        case 'date':
            this.$emit('sortFunction', this.DATE_DIRECTION);
            this.DATE_DIRECTION = this.DATE_DIRECTION === 'date-ascending' ? 'date-descending' : 'date-ascending';
            this.arrowController = { date: true, amount: false, status: false };
            this.dateSortDirection = this.dateSortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
            break;
        case 'amount':
            this.$emit('sortFunction', this.AMOUNT_DIRECTION);
            this.AMOUNT_DIRECTION = this.AMOUNT_DIRECTION === 'amount-ascending' ? 'amount-descending' : 'amount-ascending';
            this.arrowController = { date: false, amount: true, status: false };
            this.amountSortDirection = this.amountSortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
            break;
        case 'status':
            this.$emit('sortFunction', this.STATUS_DIRECTION);
            this.STATUS_DIRECTION = this.STATUS_DIRECTION === 'status-ascending' ? 'status-descending' : 'status-ascending';
            this.arrowController = { date: false, amount: false, status: true };
            this.statusSortDirection = this.statusSortDirection === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
            break;
        default:
            break;
        }
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

    .date,
    .amount,
    .status {
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
