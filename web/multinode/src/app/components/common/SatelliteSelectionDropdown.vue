// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="trustedSatellitesOptions" />
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VDropdown, { Option } from '@/app/components/common/VDropdown.vue';
import NodesTable from '@/app/components/myNodes/tables/NodesTable.vue';

import { NodeURL } from '@/nodes';

@Component({
    components: { VDropdown, NodesTable },
})
export default class SatelliteSelectionDropdown extends Vue {
    /**
     * List of trusted satellites and all satellites options.
     */
    public get trustedSatellitesOptions(): Option[] {
        const trustedSatellites: NodeURL[] = this.$store.state.nodes.trustedSatellites;

        const options: Option[] = trustedSatellites.map(
            (satellite: NodeURL) => {
                return new Option(satellite.id, () => this.onSatelliteClick(satellite.id));
            },
        );

        return [ new Option('All Satellites', () => this.onSatelliteClick()), ...options ];
    }

    /**
     * Callback for satellite click.
     * @param id
     */
    public async onSatelliteClick(id: string = ''): Promise<void> {
        await this.$store.dispatch('nodes/selectSatellite', id);
    }
}
</script>
