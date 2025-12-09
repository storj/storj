// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table>
        <template #head>
            <thead>
                <tr>
                    <th class="align-left">NAME</th>
                    <th>DISK</th>
                    <th>BANDWIDTH</th>
                    <th>PAYOUT</th>
                </tr>
            </thead>
        </template>

        <template #body>
            <tbody>
                <tr class="table-item">
                    <th class="align-left">Download</th>
                    <th>--</th>
                    <th>{{ Size.toBase10String(paystub.usageGet) }}</th>
                    <th>{{ Currency.dollarsFromCents(paystub.compGet) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">Download Repair</th>
                    <th>--</th>
                    <th>{{ Size.toBase10String(paystub.repairAndAuditUsage) }}</th>
                    <th>{{ Currency.dollarsFromCents(paystub.repairAndAuditComp) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">Disk Space</th>
                    <th>{{ Size.toBase10String(paystub.usageAtRest) }}m</th>
                    <th>--</th>
                    <th>{{ Currency.dollarsFromCents(paystub.compAtRest) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">Gross Total</th>
                    <th /><th />
                    <th>{{ Currency.dollarsFromCents(paystub.gross) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">Held amount</th>
                    <th /><th />
                    <th>{{ Currency.dollarsFromCents(paystub.held) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">NET TOTAL</th>
                    <th /><th />
                    <th>{{ Currency.dollarsFromCents(paystub.paid) }}</th>
                </tr>
                <tr class="table-item">
                    <th class="align-left">Distributed</th>
                    <th /><th />
                    <th>{{ Currency.dollarsFromCents(paystub.distributed) }}</th>
                </tr>
            </tbody>
        </template>
    </base-table>
</template>

<script setup lang="ts">
import { Paystub } from '@/payouts';
import { Currency } from '@/app/utils/currency';
import { Size } from '@/private/memory/size';

import BaseTable from '@/app/components/common/BaseTable.vue';

defineProps<{
    paystub: Paystub;
}>();
</script>
