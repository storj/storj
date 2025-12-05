// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table>
        <thead slot="head">
            <tr>
                <th class="align-left">NAME</th>
                <th>DISK</th>
                <th>BANDWIDTH</th>
                <th>PAYOUT</th>
            </tr>
        </thead>
        <tbody slot="body">
            <tr class="table-item">
                <th class="align-left">Download</th>
                <th>--</th>
                <th>{{ paystub.usageGet | bytesToBase10String }}</th>
                <th>{{ paystub.compGet | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Download Repair</th>
                <th>--</th>
                <th>{{ paystub.repairAndAuditUsage | bytesToBase10String }}</th>
                <th>{{ paystub.repairAndAuditComp | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Disk Space</th>
                <th>{{ paystub.usageAtRest | bytesToBase10String }}m</th>
                <th>--</th>
                <th>{{ paystub.compAtRest | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Gross Total</th>
                <th /><th />
                <th>{{ paystub.gross | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Held amount</th>
                <th /><th />
                <th>{{ paystub.held | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">NET TOTAL</th>
                <th /><th />
                <th>{{ paystub.paid | centsToDollars }}</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Distributed</th>
                <th /><th />
                <th>{{ paystub.distributed | centsToDollars }}</th>
            </tr>
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { Paystub } from '@/payouts';

import BaseTable from '@/app/components/common/BaseTable.vue';

// @vue/component
@Component({
    components: {
        BaseTable,
    },
})
export default class PayoutsByNodeTable extends Vue {
    @Prop({ default: () => new Paystub() })
    public paystub: Paystub;
}
</script>
