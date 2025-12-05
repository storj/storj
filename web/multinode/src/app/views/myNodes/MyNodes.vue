// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-nodes">
        <h1 class="my-nodes__title">My Nodes</h1>
        <satellite-selection-dropdown />
        <nodes-table class="table" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { UnauthorizedError } from '@/api';

import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import NodesTable from '@/app/components/myNodes/tables/NodesTable.vue';

// @vue/component
@Component({
    components: {
        SatelliteSelectionDropdown,
        NodesTable,
    },
})
export default class MyNodes extends Vue {
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('nodes/fetch');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }
}
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
            color: var(--v-header-base);
            margin-bottom: 36px;
        }
    }

    .table {
        margin-top: 20px;
    }
</style>
