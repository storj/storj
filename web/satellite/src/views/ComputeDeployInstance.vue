// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Deploy New Instance" />
        <PageSubtitleComponent subtitle="Configure and deploy a new virtual machine from the selection below." />

        <v-card variant="flat" border text-color="info" class="mt-5 mb-2 rounded-xlg">
            <v-tabs
                v-model="tab"
                color="info"
                grow
            >
                <v-tab value="on-demand">
                    On-demand Instances
                </v-tab>
                <v-tab value="reserved">
                    Reserved Instances
                </v-tab>
            </v-tabs>
        </v-card>

        <v-window v-model="tab">
            <v-window-item value="on-demand">
                <ComputeInstanceTypesTableComponent />
            </v-window-item>
            <v-window-item value="reserved">
                <ComputeInstanceTypesTableComponent />
            </v-window-item>
        </v-window>
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VCard,
    VTabs,
    VTab,
    VWindow,
    VWindowItem,
} from 'vuetify/components';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ComputeInstanceTypesTableComponent from '@/components/ComputeInstanceTypesTableComponent.vue';

const tab = ref<'reserved' | 'on-demand'>('on-demand');
</script>
