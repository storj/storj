// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="480px"
        persistent
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
                        <component :is="CalendarCheck" :size="18" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Plan confirmed
                </v-card-title>

                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="onGoToDashboard"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-text class="pa-6">
                <div class="d-flex align-center ga-2">
                    <v-icon :icon="Check" size="18" />
                    <span class="text-body-medium">Your plan changes on July 1, 2026</span>
                </div>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-btn
                    color="primary"
                    variant="flat"
                    size="large"
                    block
                    :append-icon="ArrowRight"
                    @click="onGoToDashboard"
                >
                    Go to dashboard
                </v-btn>
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
    VDialog,
    VDivider,
    VIcon,
    VSheet,
} from 'vuetify/components';
import { ArrowRight, CalendarCheck, Check, X } from '@lucide/vue';

const model = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();

function onGoToDashboard(): void {
    emit('confirm');
}
</script>
