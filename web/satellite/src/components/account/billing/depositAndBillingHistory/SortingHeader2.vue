// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th @click="sortFunction('date')">
            <div class="th-content">
                <span>DATE</span>
                <VerticalArrows
                    :is-active="arrowController.date"
                    :direction="dateSortDirection"
                />
            </div>
        </th>
        <th>
            <div class="th-content">
                <span>TRANSACTION</span>
            </div>
        </th>
        <th @click="sortFunction('amount')">
            <div class="th-content">
                <span>AMOUNT(USD)</span>
                <VerticalArrows
                    :is-active="arrowController.amount"
                    :direction="amountSortDirection"
                />
            </div>
        </th>
        <th @click="sortFunction('status')">
            <div class="th-content">
                <span>STATUS</span>
                <VerticalArrows
                    :is-active="arrowController.status"
                    :direction="statusSortDirection"
                />
            </div>
        </th>
        <th class="laptop">
            <div class="th-content">
                <span>DETAILS</span>
            </div>
        </th>
    </fragment>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { Fragment } from 'vue-fragment';

import { SortDirection } from '@/types/common';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

// @vue/component
@Component({
    components: {
        VerticalArrows,
        Fragment,
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
.th-content {
    display: flex;
    text-align: left;
}

@media screen and (max-width: 1024px) and (min-width: 426px) {

    .laptop {
        display: none;
    }
}
</style>
