// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="trustedSatellitesOptions" :preselected-option="selectedSatelliteOption" />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { NodeURL } from '@/nodes';
import { Option } from '@/app/types/common';
import { useNodesStore } from '@/app/store/nodesStore';

import VDropdown from '@/app/components/common/VDropdown.vue';

const nodesStore = useNodesStore();

const trustedSatellitesOptions = computed<Option[]>(() => {
    const trustedSatellites: NodeURL[] = nodesStore.state.trustedSatellites;

    const options: Option[] = trustedSatellites.map(
        (satellite: NodeURL) => new Option(satellite.id, () => onSatelliteClick(satellite.id)),
    );

    return [new Option('All Satellites', () => onSatelliteClick()), ...options];
});

const selectedSatelliteOption = computed<Option | null>(() => {
    if (!nodesStore.state.selectedSatellite) { return null; }

    return new Option(nodesStore.state.selectedSatellite.id, async () => Promise.resolve());
});

async function onSatelliteClick(id = ''): Promise<void> {
    await nodesStore.selectSatellite(id);
}
</script>
