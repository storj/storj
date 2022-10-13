// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th class="align-left">DATE</th>
        <th class="align-left">STATUS</th>
        <th class="align-left">AMOUNT</th>
        <th class="align-left">DOWNLOAD</th>
    </fragment>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { Fragment } from 'vue-fragment';

// @vue/component
@Component({
    components: {
        Fragment,
    },
})
export default class BillingHistoryHeader extends Vue {
}
</script>
