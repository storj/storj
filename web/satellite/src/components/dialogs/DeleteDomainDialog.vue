// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="isLoading"
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
                        <component :is="Trash2" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">
                    Delete Domain
                </v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-6">
                <p class="mb-3">
                    The following domain will be deleted.
                </p>
                <v-chip :title="domainName" class="font-weight-bold text-wrap h-100 py-2">
                    {{ domainName }}
                </v-chip>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="onDeleteClick">
                            Delete
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
} from 'vuetify/components';
import { Trash2, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useDomainsStore } from '@/store/modules/domainsStore';

const domainsStore = useDomainsStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    domainName?: string;
}>();

const emit = defineEmits<{
    'deleted': [];
}>();

const model = defineModel<boolean>({ required: true });

async function onDeleteClick(): Promise<void> {
    await withLoading(async () => {
        if (!props.domainName) {
            notify.error('Domain name is required to delete a domain');
            return;
        }

        try {
            await domainsStore.deleteDomain(props.domainName);
            notify.success(`Domain deleted successfully`);
            emit('deleted');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CONFIRM_DELETE_DOMAIN_MODAL);
        }
    });
}
</script>
