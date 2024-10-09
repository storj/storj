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
import { Notify } from '@/app/plugins';

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

    public notify = new Notify();

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('nodes/fetch');
        } catch (error: any) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            this.notify.error({ message: error.message, title: error.name });

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
            color: var(--c-title);
            margin-bottom: 36px;
        }
    }

    .table {
        margin-top: 20px;
    }
</style>
