// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row justify="center">
        <v-col class="text-center pt-10 pb-4">
            <img height="50" width="50" src="@/assets/icon-change-password.svg" alt="Change password">
            <div class="text-overline mt-2 mb-1">
                Project Encryption
            </div>
            <h2>Choose the encryption method for your data</h2>
        </v-col>
    </v-row>

    <v-row justify="center">
        <v-col cols="12" sm="8" md="6" lg="4">
            <v-card variant="outlined" rounded="xlg" class="h-100">
                <div class="d-flex flex-column justify-space-between pa-6">
                    <p>
                        <v-chip
                            rounded="lg"
                            class="text-caption font-weight-bold"
                            color="primary"
                            size="small"
                            :prepend-icon="mdiCheck"
                        >
                            Recommended
                        </v-chip>
                    </p>
                    <h3 class="font-weight-black mt-2 mb-1 text-primary">Storj Managed Encryption</h3>
                    <p class="text-medium-emphasis text-caption mb-3"> Secure, automatic encryption handled by Storj</p>

                    <v-alert color="primary" variant="tonal" rounded="lg">
                        <p class="text-body-2 mb-2">
                            Seamless collaboration: Easily work on projects with team members, no passphrase sharing required
                        </p>
                        <v-divider />
                        <p class="text-body-2 my-2">
                            Effortless security: Storj automatically encrypts and decrypts your data, ensuring robust protection
                        </p>
                        <v-divider />
                        <p class="text-body-2 mt-2">
                            Streamlined workflow: Fewer steps to upload and download files, simplifying your experience
                        </p>
                    </v-alert>

                    <v-btn
                        :disabled="selectedMode !== 'auto' && loading"
                        :loading="selectedMode === 'auto' && loading"
                        class="mt-4"
                        @click="emit('modeChosen', 'auto')"
                    >
                        <template #append>
                            <v-icon :icon="mdiArrowRight" />
                        </template>
                        Storj Managed Encryption
                    </v-btn>
                </div>
            </v-card>
        </v-col>

        <v-col cols="12" sm="8" md="6" lg="4">
            <v-card variant="outlined" rounded="xlg" class="h-100">
                <div class="d-flex flex-column justify-space-between pa-6">
                    <p><v-chip rounded="lg" class="text-caption font-weight-bold" color="purple" size="small">Advanced</v-chip></p>
                    <h3 class="font-weight-black mt-2 mb-1 primary">Self-Managed Encryption</h3>
                    <p class="text-medium-emphasis text-caption mb-3">Full control over your encryption keys</p>
                    <v-alert color="secondary" variant="tonal" rounded="lg">
                        <p class="text-body-2 mb-2">
                            Enhanced privacy: You have complete ownership and control over your encryption passphrase
                        </p>

                        <v-divider />

                        <p class="text-body-2 my-2">
                            Client-side encryption: Your data is encrypted before reaching our servers, ensuring maximum confidentiality
                        </p>

                        <v-divider />

                        <p class="text-body-2 mt-2">
                            Responsibility: Securely manage and store your passphrase, as Storj cannot recover it if lost
                        </p>
                    </v-alert>

                    <v-btn
                        :disabled="selectedMode !== 'manual' && loading"
                        :loading="selectedMode === 'manual' && loading"
                        color="secondary"
                        class="mt-4"
                        @click="emit('modeChosen', 'manual')"
                    >
                        Self-Managed Encryption
                        <template #append>
                            <v-icon :icon="mdiArrowRight" />
                        </template>
                    </v-btn>
                </div>
            </v-card>
        </v-col>
    </v-row>

    <v-row justify="center" class="mt-3">
        <v-col cols="12" sm="10">
            <p class="text-body-2 mt-2 text-center">
                <a href="" class="link">Learn more about encryption methods.</a>
            </p>
        </v-col>
    </v-row>

    <v-row justify="center" class="mt-3">
        <v-col cols="6" sm="4" md="3" lg="2">
            <v-btn
                :disabled="loading"
                size="small"
                variant="text"
                :prepend-icon="mdiChevronLeft"
                color="default" block
                @click="emit('back')"
            >
                Back
            </v-btn>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VChip, VCol, VDivider, VIcon, VRow } from 'vuetify/components';
import { mdiArrowRight, mdiCheck, mdiChevronLeft } from '@mdi/js';

import { ManagePassphraseMode } from '@/types/projects';

defineProps<{
    loading: boolean
    selectedMode?: ManagePassphraseMode
}>();

const emit = defineEmits<{
    modeChosen: [ManagePassphraseMode]
    back: []
}>();
</script>
