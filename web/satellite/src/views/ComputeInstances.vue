// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Your Instances" />
        <PageSubtitleComponent subtitle="Manage your existing virtual machine instances." />

        <v-row class="mt-2 mb-4">
            <v-col>
                <v-btn
                    :to="ROUTES.ComputeDeployInstance.path"
                    color="primary"
                    :append-icon="Plus"
                    class="font-weight-bold"
                >
                    Deploy New Instance
                </v-btn>
            </v-col>
        </v-row>

        <ComputeInstancesTableComponent class="mt-5" />
    </v-container>
</template>

<script setup lang="ts">
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';
import { Plus } from 'lucide-vue-next';

import { ROUTES } from '@/router';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ComputeInstancesTableComponent from '@/components/ComputeInstancesTableComponent.vue';
</script>
