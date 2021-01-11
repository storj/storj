// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-nodes">
        <h1 class="my-nodes__title">My Nodes</h1>
        <nodes-table />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NodesTable from '@/app/components/tables/NodesTable.vue';

@Component({
    components: { NodesTable },
})
export default class MyNodes extends Vue {}
</script>

<style lang="scss" scoped>
    .my-nodes {
        box-sizing: border-box;
        padding: 60px;
        height: 100%;
        overflow-y: auto;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--c-title);
            margin-bottom: 36px;
        }
    }
</style>
