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
                        <component :is="CircleQuestionMark" :size="18" color="orange" />
                    </v-sheet>
                </template>

                <v-card-title class="font-weight-bold">
                    Not ready to decide yet?
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
                You can decide later, but a choice will be required by the end of June 30, 2026.
                If no selection is made by then, your account will be suspended on July 1, 2026.
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
                            color="primary"
                            variant="flat"
                            block
                            @click="emit('confirm')"
                        >
                            Confirm
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
import { CircleQuestionMark, X } from '@lucide/vue';

const model = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();
</script>
