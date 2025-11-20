// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        activator="parent"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card>
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <img src="@/assets/icon-color-globe.svg" alt="Earth" width="40" class="mt-1">
                    </template>
                    <v-card-title class="font-weight-bold">
                        {{ configStore.brandName }} Sustainability
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-6">
                <p class="text-body-2 mb-4">
                    The carbon emissions displayed are estimated based on the total usage of this project, calculated from the date of project creation up to the present day.
                </p>
                <v-card class="pa-4 mb-4">
                    <p class="text-body-2 font-weight-bold mb-2">Carbon Emissions</p>
                    <v-chip variant="tonal" color="info" class="font-weight-bold">
                        {{ storjImpact.toLocaleString() }} kg CO₂e
                    </v-chip>
                    <p class="text-body-2 mt-2">Estimated for this {{ configStore.brandName }} project. <a href="https://www.storj.io/documents/storj-sustainability-whitepaper.pdf" target="_blank" rel="noopener noreferrer" class="link">Learn more</a></p>
                </v-card>
                <v-card class="pa-4 mb-4">
                    <p class="text-body-2 font-weight-bold mb-2">Carbon Comparison</p>
                    <v-chip variant="tonal" color="warning" class="font-weight-bold">
                        {{ hyperscalerImpact.toLocaleString() }} kg CO₂e
                    </v-chip>
                    <p class="text-body-2 mt-2">By using traditional cloud storage. <a href="https://www.storj.io/documents/storj-sustainability-whitepaper.pdf" target="_blank" rel="noopener noreferrer" class="link">Learn more</a></p>
                </v-card>
                <v-card class="pa-4 mb-4">
                    <p class="text-body-2 font-weight-bold mb-2">Total Carbon Avoided</p>
                    <v-chip variant="tonal" color="success" class="font-weight-bold">
                        {{ co2Savings }} kg CO₂e
                    </v-chip>
                    <p class="text-body-2 mt-2">Estimated by using {{ configStore.brandName }}. <a href="https://www.storj.io/documents/storj-sustainability-whitepaper.pdf" target="_blank" rel="noopener noreferrer" class="link">Learn more</a></p>
                </v-card>
                <v-card class="pa-4 mb-2">
                    <p class="text-body-2 font-weight-bold mb-2">Carbon Avoided Equals To</p>
                    <v-chip variant="tonal" color="success" class="font-weight-bold">
                        {{ emission.savedTrees.toLocaleString() }} tree{{ emission.savedTrees !== 1 ? 's' : '' }} grown for 10 years
                    </v-chip>
                    <p class="text-body-2 mt-2">Estimated equivalencies. <a href="https://www.epa.gov/energy/greenhouse-gases-equivalencies-calculator-calculations-and-references#seedlings" target="_blank" rel="noopener noreferrer" class="link">Learn more</a></p>
                </v-card>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn color="primary" variant="flat" block link href="https://www.storj.io/documents/storj-sustainability-whitepaper.pdf" target="_blank" rel="noopener noreferrer">
                            Sustainability Whitepaper <v-icon :icon="SquareArrowOutUpRight" class="ml-2" />
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardActions,
    VCardTitle,
    VCardText,
    VBtn,
    VChip,
    VRow,
    VCol,
    VIcon,
    VDivider,
} from 'vuetify/components';
import { SquareArrowOutUpRight, X } from 'lucide-vue-next';

import { Emission } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const model = ref<boolean>(false);

/**
 * Returns project's emission impact.
 */
const emission = computed<Emission>(()  => {
    return projectsStore.state.emission;
});

/**
 * Returns project's estimated hyperscaler emission impact.
 */
const hyperscalerImpact = computed<number>(() => {
    return Math.round(emission.value.hyperscalerImpact);
});

/**
 * Returns project's estimated storj emission impact.
 */
const storjImpact = computed<number>(() => {
    return Math.round(emission.value.storjImpact);
});

/**
 * Returns calculated and formatted CO2 savings info.
 */
const co2Savings = computed<string>(() => {
    let saved = hyperscalerImpact.value - storjImpact.value;
    if (saved < 0) saved = 0;

    return saved.toLocaleString();
});
</script>
