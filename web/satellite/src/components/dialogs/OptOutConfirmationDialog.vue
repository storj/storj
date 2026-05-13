// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="480px"
        transition="fade-transition"
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
                        <component :is="TriangleAlert" :size="18" color="orange" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Are you sure you want to opt-out?
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

            <v-divider />

            <v-card-text>
                If you choose not to accept the new pricing, your account will be scheduled for
                suspension. You'll still have access to download all of your data. To keep using
                Storj, please accept the updated pricing.
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            color="default"
                            variant="outlined"
                            block
                            @click="model = false"
                        >
                            Go back
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="error"
                            variant="flat"
                            block
                            @click="emit('confirm')"
                        >
                            Confirm opt out
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
    VCardText,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { TriangleAlert, X } from 'lucide-vue-next';

const model = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();
</script>
