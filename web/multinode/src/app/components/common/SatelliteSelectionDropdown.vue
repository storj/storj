// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dropdown :options="trustedSatellitesOptions" :preselected-option="selectedSatelliteOption" />
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { NodeURL } from '@/nodes';
import { useStore } from '@/app/utils/composables';
import { Option } from '@/app/types/common';

import VDropdown from '@/app/components/common/VDropdown.vue';

const store = useStore();

const trustedSatellitesOptions = computed<Option[]>(() => {
    const trustedSatellites: NodeURL[] = store.state.nodes.trustedSatellites;

    const options: Option[] = trustedSatellites.map(
        (satellite: NodeURL) => new Option(satellite.id, () => onSatelliteClick(satellite.id)),
    );

    return [new Option('All Satellites', () => onSatelliteClick()), ...options];
});

const selectedSatelliteOption = computed<Option | null>(() => {
    if (!store.state.nodes.selectedSatellite) { return null; }

    return new Option(store.state.nodes.selectedSatellite.id, async () => Promise.resolve());
});

async function onSatelliteClick(id = ''): Promise<void> {
    await store.dispatch('nodes/selectSatellite', id);
}
</script>
