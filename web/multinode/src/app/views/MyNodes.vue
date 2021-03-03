// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="my-nodes">
        <h1 class="my-nodes__title">My Nodes</h1>
        <v-dropdown :options="trustedSatellitesOptions" />
        <nodes-table class="table"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VDropdown, { Option } from '@/app/components/common/VDropdown.vue';
import NodesTable from '@/app/components/tables/NodesTable.vue';

import { UnauthorizedError } from '@/api';
import { NodeURL } from '@/nodes';

@Component({
components: { VDropdown, NodesTable },
})
export default class MyNodes extends Vue {
    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('nodes/trustedSatellites');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }
            // TODO: notify error
        }

        try {
            await this.$store.dispatch('nodes/fetch');
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }

    public get trustedSatellitesOptions(): Option[] {
        const trustedSatellites: NodeURL[] = this.$store.state.nodes.trustedSatellites;

        const options: Option[] = trustedSatellites.map(
            (satellite: NodeURL) => {
                return new Option(satellite.id, () => this.onSatelliteClick(satellite.id));
            },
        );

        return [ new Option('All Satellites', () => this.onSatelliteClick()), ...options ];
    }

    public async onSatelliteClick(id: string = ''): Promise<void> {
        await this.$store.dispatch('nodes/selectSatellite', id);
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
