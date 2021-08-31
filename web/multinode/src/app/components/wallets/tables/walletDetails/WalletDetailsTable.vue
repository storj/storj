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
                <th>{{ 2000 | centsToDollars }}</th>
            </tr>
        </tbody>
    </base-table>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BaseTable from '@/app/components/common/BaseTable.vue';

// @vue/component
@Component({
    components: { BaseTable },
})
export default class WalletDetailsTable extends Vue {}
</script>
