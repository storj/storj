// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table>
        <thead slot="head">
            <tr>
                <th class="align-left">NODE</th>
                <th>UNDISTRIBUTED</th>
            </tr>
        </thead>
        <tbody slot="body">
            <tr class="table-item">
                <th class="align-left">My New Node</th>
                <th>{{ Currency.dollarsFromCents(2000) }}</th>
            </tr>
        </tbody>
    </base-table>
</template>

<script setup lang="ts">
import { Currency } from '@/app/utils/currency';

import BaseTable from '@/app/components/common/BaseTable.vue';
</script>
