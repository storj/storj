// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="12" md="6" lg="6" xl="3">
        <v-card class="px-2 py-4">
            <v-card-item class="pb-0">
                <img :src="app.src" :alt="app.title" width="42" class="rounded">
            </v-card-item>

            <v-card-item>
                <v-chip size="small" variant="tonal" color="default" class="mb-3 font-weight-medium" rounded>
                    {{ app.category }}
                </v-chip>
                <h3 class="mb-1">
                    {{ app.title }}
                </h3>
                <p class="mt-1 text-medium-emphasis">
                    {{ app.description }}
                </p>
            </v-card-item>

            <v-card-item class="bottom">
                <v-btn color="primary" @click="onSetup">
                    Setup
                    <template #append>
                        <v-icon :icon="mdiArrowRight" />
                    </template>
                </v-btn>
                <v-btn variant="outlined" color="default" class="ml-2" :href="app.docs" target="_blank" rel="noopener noreferrer">
                    Docs
                    <template #append>
                        <v-icon :icon="mdiOpenInNew" />
                    </template>
                </v-btn>
            </v-card-item>
        </v-card>
    </v-col>
    <CreateAccessDialog ref="accessDialog" v-model="dialog" :default-name="app.title" />
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VBtn, VCard, VCardItem, VChip, VCol, VIcon } from 'vuetify/components';
import { mdiArrowRight, mdiOpenInNew } from '@mdi/js';

import { Application } from '@/types/applications';
import { AccessType, Exposed } from '@/types/createAccessGrant';

import CreateAccessDialog from '@/components/dialogs/CreateAccessDialog.vue';

const props = defineProps<{
    app: Application
}>();

const accessDialog = ref<Exposed>();
const dialog = ref<boolean>(false);

/**
 * Holds on setup button click logic.
 * Starts create S3 credentials flow.
 */
function onSetup(): void {
    accessDialog.value?.setTypes([AccessType.S3]);
    dialog.value = true;
}
</script>
