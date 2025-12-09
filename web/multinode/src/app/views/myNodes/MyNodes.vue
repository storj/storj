// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-nodes">
        <h1 class="my-nodes__title">My Nodes</h1>
        <satellite-selection-dropdown />
        <nodes-table class="table" />
    </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue';

import { UnauthorizedError } from '@/api';
import { useNodesStore } from '@/app/store/nodesStore';

import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import NodesTable from '@/app/components/myNodes/tables/NodesTable.vue';

const nodesStore = useNodesStore();

onMounted(async () => {
    try {
        await nodesStore.fetch();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }
});
</script>

<style lang="scss" scoped>
    .my-nodes {
        box-sizing: border-box;
        padding: 60px;
        height: 100%;
        overflow-y: auto;
        background-color: var(--v-background-base);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--v-header-base);
            margin-bottom: 36px;
        }
    }

    .table {
        margin-top: 20px;
    }
</style>
