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
                <template v-if="!externalAuthEnabled">
                    You can change your session timeout preferences in your account settings.
                </template>
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
import { computed } from 'vue';
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
import { useConfigStore } from '@/store/modules/configStore';

const router = useRouter();
const configStore = useConfigStore();

const model = defineModel<boolean>({ required: true });

const externalAuthEnabled = computed<boolean>(() => configStore.externalAuthEnabled);

/**
 * Redirects to login screen.
 */
function redirectToLogin(): void {
    if (externalAuthEnabled.value) {
        window.location.href = configStore.state.config.primaryAuthLoginURL;
    } else {
        router.push(ROUTES.Login.path);
    }
    model.value = false;
}
</script>

<style scoped lang="scss">
.custom-dialog-overlay {
    backdrop-filter: blur(5px);
    background-color: rgb(255 255 255 / 50%);
}
</style>
