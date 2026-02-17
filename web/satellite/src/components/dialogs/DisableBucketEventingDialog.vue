// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card :loading="isLoading">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="BellOff" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Disable Bucket Eventing
                </v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-6">
                Are you sure you want to disable bucket eventing for <strong>{{ props.bucketName }}</strong>?
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            :disabled="isLoading"
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="warning"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="save()"
                        >
                            Disable
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
import { BellOff } from 'lucide-vue-next';

import { useEventing } from '@/composables/useEventing';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';

const {  updateNotificationConfig } = useEventing();
const { withLoading, isLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    bucketName: string;
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    disabled: [];
}>();

function save() {
    withLoading(async () => {
        try {
            await updateNotificationConfig(props.bucketName, null);
            notify.success(`Bucket eventing configuration disabled successfully`);
            model.value = false;
            emit('disabled');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.DISABLE_BUCKET_EVENTING_CONFIG_DIALOG);
        }
    });
}
</script>
