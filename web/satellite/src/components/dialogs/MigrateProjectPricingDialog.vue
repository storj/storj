// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="CircleFadingArrowUp" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold"> Migrate Project </v-card-title>
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

            <v-divider />

            <v-card-item class="px-6">
                <v-card-text class="pa-0 mb-4">
                    We'll update this project to the new storage tiering model.
                    New buckets will use the latest tiers.
                    Existing buckets stay where they are.
                    No data is moved.
                </v-card-text>

                <v-expansion-panels variant="accordion">
                    <v-expansion-panel elevation="0" static rounded class="border-sm">
                        <v-expansion-panel-title class="font-weight-bold">
                            What changes
                        </v-expansion-panel-title>
                        <v-expansion-panel-text>
                            New storage tier options: Global Collaboration,
                            {{ isUSSatellite ? 'Regional US, ' : '' }}
                            Active Archive.
                            <br><br>
                            Existing buckets usage will be billed as:
                            <br>
                            Legacy Global → Active Archive
                            <template v-if="isUSSatellite">
                                <br>
                                Legacy Select → Regional US
                            </template>
                            <br><br>
                            This change applies for the whole current billing period of one month.
                        </v-expansion-panel-text>
                    </v-expansion-panel>
                </v-expansion-panels>

                <v-card-text class="pa-0 mt-4">
                    This change is permanent and cannot be undone.
                    Pricing will follow the new tiers.
                    <a href="https://storj.dev/dcs/pricing" target="_blank" rel="noopener noreferrer">Pricing and tiers.</a>
                </v-card-text>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="migrate"
                        >
                            Migrate Project
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
    VCardText,
    VExpansionPanels,
    VExpansionPanel,
    VExpansionPanelTitle,
    VExpansionPanelText,
} from 'vuetify/components';
import { CircleFadingArrowUp, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

const projectsStore = useProjectsStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    projectId: string;
}>();

const emit = defineEmits<{
    'success': [];
}>();

const model = defineModel<boolean>({ required: true });

const isUSSatellite = computed<boolean>(() => configStore.state.config.satelliteName === 'US1');

function migrate(): void {
    withLoading(async () => {
        try {
            await projectsStore.migratePricing(props.projectId);
            await projectsStore.getProjects();

            notify.success('Project migrated successfully');

            emit('success');
            model.value = false;
        } catch (error) {
            notify.notifyError(error);
        }
    });
}
</script>
