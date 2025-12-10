// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="Your Instances" />
        <PageSubtitleComponent subtitle="Manage your existing virtual machine instances." />

        <v-row class="mt-2 mb-4">
            <v-col>
                <v-btn
                    color="primary"
                    :append-icon="Plus"
                    class="font-weight-bold"
                    @click="isCreateDialog = true"
                >
                    Deploy New Instance
                </v-btn>
            </v-col>
        </v-row>

        <ComputeInstancesTableComponent />
    </v-container>

    <create-compute-instance-dialog v-model="isCreateDialog" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';
import { Plus } from 'lucide-vue-next';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ComputeInstancesTableComponent from '@/components/ComputeInstancesTableComponent.vue';
import CreateComputeInstanceDialog from '@/components/dialogs/CreateComputeInstanceDialog.vue';

const isCreateDialog = ref<boolean>(false);
</script>
