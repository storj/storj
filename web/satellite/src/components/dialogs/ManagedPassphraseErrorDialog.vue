// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center text-error"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="ShieldAlert" :size="20" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold text-error">
                        Error
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

            <v-card-item class="pa-6">
                <p class="mb-3 font-weight-bold">
                    Your project can't be unlocked at the moment. Don't worry, your objects are safe and secure. Please try again later.
                </p>
                <p class="mb-3">
                    If the problem continues, our support team is ready to assist you at support@storj.io
                </p>
                <v-alert>
                    Error: Could not retrieve managed passphrase.
                </v-alert>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block @click="model = false">
                            Close
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VAlert,
    VDialog,
    VSheet,
    VCard,
    VCardItem,
    VCardTitle,
    VCardActions,
    VRow,
    VCol,
    VDivider,
    VBtn,
} from 'vuetify/components';
import { ShieldAlert, X } from 'lucide-vue-next';

const model = defineModel<boolean>({ required: true });
</script>