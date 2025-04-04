// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        persistent
        class="custom-dialog-overlay"
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
                        <component :is="TimerReset" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Session Expired</v-card-title>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-6">
                To protect your account and data, you've been automatically logged out.
                You can change your session timeout preferences in your account settings.
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
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
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VBtn,
    VDivider,
    VCardActions,
    VSheet,
} from 'vuetify/components';
import { TimerReset } from 'lucide-vue-next';

import { ROUTES } from '@/router';

const router = useRouter();

const model = defineModel<boolean>({ required: true });

/**
 * Redirects to login screen.
 */
function redirectToLogin(): void {
    router.push(ROUTES.Login.path);
    model.value = false;
}
</script>

<style scoped lang="scss">
.custom-dialog-overlay {
    backdrop-filter: blur(5px);
    background-color: rgb(255 255 255 / 50%);
}
</style>
