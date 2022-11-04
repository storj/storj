// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="trustedSatellitesOptions" :preselected-option="selectedSatelliteOption" />
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { NodeURL } from '@/nodes';

import VDropdown, { Option } from '@/app/components/common/VDropdown.vue';

// @vue/component
@Component({
    components: { VDropdown },
})
export default class SatelliteSelectionDropdown extends Vue {
    /**
     * List of trusted satellites and all satellites options.
     */
    public get trustedSatellitesOptions(): Option[] {
        const trustedSatellites: NodeURL[] = this.$store.state.nodes.trustedSatellites;

        const options: Option[] = trustedSatellites.map(
            (satellite: NodeURL) => new Option(satellite.id, () => this.onSatelliteClick(satellite.id)),
        );

        return [new Option('All Satellites', () => this.onSatelliteClick()), ...options];
    }

    /**
     * Preselected satellite from store if any.
     */
    public get selectedSatelliteOption(): Option | null {
        if (!this.$store.state.nodes.selectedSatellite) { return null; }

        return new Option(this.$store.state.nodes.selectedSatellite.id, async() => Promise.resolve());
    }

    /**
     * Callback for satellite click.
     * @param id
     */
    public async onSatelliteClick(id = ''): Promise<void> {
        await this.$store.dispatch('nodes/selectSatellite', id);
    }
}
</script>
