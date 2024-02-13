// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        persistent
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/assets/icon-session-timeout.svg" alt="Session expired">
                </template>
                <v-card-title class="font-weight-bold">Session Expired</v-card-title>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-8">
                To protect your account and data, you've been automatically logged out.
                You can change your session timeout preferences in your account settings.
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-7">
                <v-btn
                    color="primary"
                    variant="flat"
                    block
                    @click="redirectToLogin"
                >
                    Go to login
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';
import { VDialog, VCard, VCardItem, VCardTitle, VBtn, VDivider, VCardActions } from 'vuetify/components';

import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';

const analyticsStore = useAnalyticsStore();
const router = useRouter();

const model = defineModel<boolean>({ required: true });

/**
 * Redirects to login screen.
 */
function redirectToLogin(): void {
    analyticsStore.pageVisit(ROUTES.Login.path);
    router.push(ROUTES.Login.path);
    model.value = false;
    // TODO this reload will be unnecessary once vuetify poc has its own login and/or becomes the primary app
    location.reload();
}
</script>
