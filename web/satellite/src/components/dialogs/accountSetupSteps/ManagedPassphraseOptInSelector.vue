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
                    <h3 class="font-weight-black">Automatic</h3>
                    <p>Storj securely manages the encryption and decryption of your project automatically.</p>
                    <p><v-chip rounded="md" class="text-caption mt-2 mb-4 font-weight-medium" color="secondary" variant="tonal" size="small">
                    Recommended for most users and teams
                    </v-chip></p>

                    <p class="text-body-2 my-2">
                        <b>Simple user experience</b><br>
                        Fewer steps to upload, download, manage, and browse your data.
                        No need to remember an additional encryption passphrase.
                    </p>

                    <p class="text-body-2 my-2">
                        <b>Easy team management</b><br>
                        Your team members would automatically have access to your project's data.
                    </p>
                    
                    <p class="text-body-2 my-2">
                        <a href="" class="link">Learn more in the documentation.</a>
                    </p>

                    <v-btn
                        :disabled="selectedMode !== 'auto' && loading"
                        :loading="selectedMode === 'auto' && loading"
                        class="mt-4"
                        @click="emit('modeChosen', 'auto')"
                    >
                        <template #append>
                            <v-icon :icon="mdiArrowRight" />
                        </template>
                        Automatic
                    </v-btn>
                </div>
            </v-card>
        </v-col>

        <v-col cols="12" sm="8" md="6" lg="4">
            <v-card variant="outlined" rounded="xlg" class="h-100">
                <div class="d-flex flex-column justify-space-between pa-6">
                    <h3 class="font-weight-black primary">Manual</h3>
                    <p>You are responsible for securely managing your own data encryption passphrase.</p>
                    <p>
                        <v-chip rounded="md" class="text-caption mt-2 mb-4 font-weight-medium" color="secondary" size="small" variant="tonal">
                        Best for control over your data encryption
                        </v-chip>
                    </p>

                    <p class="text-body-2 my-2">
                        <b>Passphrase experience</b><br>
                        You will need to enter your passphrase each time you access your data. 
                        If you forget the passphrase, you can't recover your data.
                    </p>

                    <p class="text-body-2 my-2">
                        <b>Manual team management</b><br>
                        Team members must share and enter the same encryption passphrase to access the data.
                    </p>

                    <p class="text-body-2 my-2">
                        <a href="" class="link">Learn more in the documentation.</a>
                    </p>

                    <v-btn
                        :disabled="selectedMode !== 'manual' && loading"
                        :loading="selectedMode === 'manual' && loading"
                        color="secondary"
                        class="mt-4"
                        @click="emit('modeChosen', 'manual')"
                    >
                        Manual
                        <template #append>
                            <v-icon :icon="mdiArrowRight" />
                        </template>
                    </v-btn>
                </div>
            </v-card>
        </v-col>
    </v-row>

    <v-row justify="center" class="mt-3">
        <v-col cols="6" sm="4" md="3" lg="2">
            <v-btn
                :disabled="loading"
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
import { VBtn, VCard, VChip, VCol, VDivider, VIcon, VRow } from 'vuetify/components';
import { mdiArrowRight, mdiChevronLeft } from '@mdi/js';

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
