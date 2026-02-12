// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="pb-12">
        <v-row class="align-center justify-space-between mt-1">
            <v-col cols="auto">
                <p><small class="text-uppercase">Compute Overview</small></p>
                <PageTitleComponent :title="title" class="mt-1 mb-2" />
                <PageSubtitleComponent subtitle="View your instances and compute usage for this month." />
            </v-col>
        </v-row>

        <v-row class="d-flex align-center justify-center mt-2">
            <v-col cols="12" sm="6" md="4">
                <v-card title="Active Instances" variant="outlined">
                    <v-card-text>
                        <v-chip color="success" class="font-weight-bold">2 Online</v-chip>
                        <v-chip color="default" class="font-weight-bold ml-2">1 Offline</v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="4">
                <v-card title="Total vCPUs" variant="outlined">
                    <v-card-text>
                        <v-chip color="default" class="font-weight-bold">84</v-chip>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="4">
                <v-card title="Compute Hours" variant="outlined">
                    <v-card-text>
                        <v-chip color="default" class="font-weight-bold">128</v-chip>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VRow,
    VCol,
    VCard,
    VCardText,
    VChip,
} from 'vuetify/components';
import { computed } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';

const configStore = useConfigStore();

const title = computed<string>(() => `My ${configStore.brandName} project`);
</script>
