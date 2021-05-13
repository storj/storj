// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table >
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
                <th>98.24 GB</th>
                <th>204.25$</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Download Repair</th>
                <th>--</th>
                <th>98.24 GB</th>
                <th>1 420.68$</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Disk Space</th>
                <th>25.45 GBm</th>
                <th>--</th>
                <th>30.77$</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Gross Total</th>
                <th></th><th></th>
                <th>163.18$</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Held amount</th>
                <th></th><th></th>
                <th>11 204.25$</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">NET TOTAL</th>
                <th></th><th></th>
                <th>$14.5</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Minimal Threshold</th>
                <th></th><th></th>
                <th>$35</th>
            </tr>
            <tr class="table-item">
                <th class="align-left">Distributed</th>
                <th></th><th></th>
                <th>$0</th>
            </tr>
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BaseTable from '@/app/components/common/BaseTable.vue';

@Component({
    components: {
        BaseTable,
    },
})
export default class PayoutsByNodeTable extends Vue {}
</script>
