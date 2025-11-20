// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Microchip" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Cloud GPUs
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

            <v-card-item>
                <p class="font-weight-bold">
                    Valdi - Cloud GPUs Powered by Storj
                </p>
            </v-card-item>
            <v-card-item>
                <p>
                    You're about to be redirected to Valdi, a division of Storj.
                    Valdi offers high-performance on-demand Cloud GPU solutions
                    designed to accelerate your workloads, at 50% the cost.
                </p>
            </v-card-item>
            <v-card-item>
                <p>
                    Please note that your {{ configStore.brandName }} account is not currently connected
                    to Valdi, and you will need to create a separate Valdi account
                    to access the service.
                </p>
            </v-card-item>
            <v-card-item>
                <p>
                    Click "Continue" to proceed to Valdi's website.
                </p>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                    </v-col>

                    <v-col>
                        <v-btn
                            variant="flat"
                            block
                            :append-icon="ExternalLink"
                            @click="goToValdi"
                        >
                            Continue
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
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
} from 'vuetify/components';
import { ExternalLink, Microchip, X } from 'lucide-vue-next';

import { AnalyticsEvent, PageVisitSource } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const userStore = useUsersStore();

const model = defineModel<boolean>({ default: false });

function goToValdi() {
    analyticsStore.pageVisit(configStore.state.config.valdiSignUpURL, PageVisitSource.VALDI);
    analyticsStore.eventTriggered(AnalyticsEvent.CLOUD_GPU_SIGN_UP_CLICKED);
    let url = configStore.state.config.valdiSignUpURL;
    url = `${url}?storj_userid=${userStore.state.user.id}`;
    url = `${url}&storj_satellite=${configStore.state.config.satelliteName}`;
    window.open(url, '_blank', 'noreferrer');
}
</script>
