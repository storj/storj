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

    <create-instance-dialog v-model="isCreateDialog" />
</template>

<script setup lang="ts">
import { onBeforeMount, ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
} from 'vuetify/components';
import { Plus } from 'lucide-vue-next';
import { useRouter } from 'vue-router';

import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import ComputeInstancesTableComponent from '@/components/ComputeInstancesTableComponent.vue';
import CreateInstanceDialog from '@/components/dialogs/compute/CreateInstanceDialog.vue';

const router = useRouter();
const configStore = useConfigStore();

const isCreateDialog = ref<boolean>(false);

onBeforeMount(() => {
    if (!configStore.isDefaultBrand) {
        router.replace({ name: ROUTES.Dashboard.name });
    }
});
</script>
