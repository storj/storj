// Copyright (C) 2023 Storj Labs, Inc.
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
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <icon-info :size="22" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Object Overwrite Warning
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="onCancel"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p class="my-2 font-weight-bold">
                        You are uploading to an unversioned bucket.
                    </p>
                    <p>
                        This non-exhaustive list of objects you are uploading have the same names as existing objects in this bucket:
                    </p>
                    <v-chip v-for="name in filenames" :key="name" class="font-weight-bold text-wrap mt-3 mr-1 py-2">
                        {{ name }}
                    </v-chip>
                    <p />
                    <v-alert color="default" variant="tonal" class="my-4">
                        If you continue with the upload, the existing object(s) will be permanently overwritten, and previous versions cannot be recovered.
                    </v-alert>
                    <v-checkbox-btn v-model="dismissPermanently" density="comfortable" class="mb-2 ml-n2" label="Do not show this warning again." />
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn :disabled="isLoading" variant="outlined" color="default" block @click="onCancel">Cancel</v-btn>
                    </v-col>
                    <v-col>
                        <v-btn :loading="isLoading" color="primary" variant="flat" block @click="onContinue">
                            Continue
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">

import { ref } from 'vue';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCheckboxBtn,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';

import IconInfo from '@/components/icons/IconInfo.vue';

const userStore = useUsersStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

withDefaults(defineProps<{
    filenames?: string[],
}>(), {
    filenames: () => [],
});

const model = defineModel<boolean>({ required: true });

const emit = defineEmits(['proceed', 'cancel']);

const dismissPermanently = ref(false);

function onContinue() {
    withLoading(async () => {
        try {
            if (dismissPermanently.value) {
                const noticeDismissal = { ...userStore.state.settings.noticeDismissal };
                noticeDismissal.uploadOverwriteWarning = true;
                await userStore.updateSettings({ noticeDismissal });
            }
            model.value = false;
            emit('proceed');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_OVERWRITE_WARNING_DIALOG);
        }
    });
}

async function onCancel() {
    emit('cancel');
    model.value = false;
}
</script>