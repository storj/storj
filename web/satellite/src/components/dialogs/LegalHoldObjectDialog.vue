// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="400px"
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="FileLock2" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        {{ hasLegalHold ? 'Remove' : '' }} Legal Hold
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

            <v-row>
                <v-col class="pa-6 mx-3">
                    <p v-if="!hasLegalHold" class="my-2">
                        Apply Legal Hold to indefinitely preserve this file,
                        preventing any changes or deletion until explicitly
                        removed by  authorized personnel for legal or
                        compliance purposes.
                    </p>
                    <p v-else class="my-2">
                        Removing Legal Hold will allow this file to be modified
                        or deleted. Ensure all legal and compliance
                        requirements have been met before proceeding.
                    </p>

                    <p class="mt-4 mb-2 font-weight-bold text-body-2">
                        Name:
                    </p>

                    <v-chip
                        variant="tonal"
                        filter
                        value="filename"
                        color="default"
                        class="mb-2 font-weight-bold"
                    >
                        {{ file?.Key }}
                    </v-chip>

                    <template v-if="file?.VersionId">
                        <p class="my-2 font-weight-bold text-body-2">
                            Version:
                        </p>

                        <v-chip
                            variant="tonal"
                            filter
                            color="default"
                            class="mb-4 font-weight-bold"
                        >
                            {{ file?.VersionId }}
                        </v-chip>
                    </template>
                </v-col>
            </v-row>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            variant="outlined"
                            color="default"
                            :disabled="isLoading"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            block
                            @click="applyLegalHold"
                        >
                            {{ hasLegalHold ? 'Remove' : 'Apply' }} Legal Hold
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VRow,
    VSheet,
} from 'vuetify/components';
import { FileLock2, X } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { LEGAL_HOLD_OFF, LEGAL_HOLD_ON } from '@/types/objectLock';

const obStore = useObjectBrowserStore();

const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const model = defineModel<boolean>({ required: false });
const props = defineProps<{
    file: BrowserObject | null
}>();

const emit = defineEmits<{
    'contentRemoved': [],
    'fileLocked': [],
}>();

const hasLegalHold = ref(false);
const innerContent = ref<VCard | null>(null);

function applyLegalHold() {
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        try {
            await obStore.legalHoldObject(props.file, hasLegalHold.value ? LEGAL_HOLD_OFF : LEGAL_HOLD_ON);
            notify.success(`Legal hold ${hasLegalHold.value ? 'removed' : 'applied'} successfully`);
            emit('fileLocked');
            model.value = false;
        } catch (e) {
            notify.notifyError(e, AnalyticsErrorEventSource.LEGAL_HOLD_DIALOG);
            return;
        }
    });
}

watch(innerContent, comp => {
    if (!comp) {
        emit('contentRemoved');
        return;
    }
    withLoading(async () => {
        if (!props.file) {
            return;
        }
        try {
            hasLegalHold.value = await obStore.getObjectLegalHold(props.file);
        } catch {
            /* empty */
        }
    });
});
</script>
