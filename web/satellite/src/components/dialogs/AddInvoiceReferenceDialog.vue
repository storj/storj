// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="400px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <v-card-title class="font-weight-bold"> Add Invoice Reference </v-card-title>
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

            <v-card-item class="px-6">
                <p class="mt-1 mb-4">Add any additional information you want to appear on your invoice here.</p>
                <v-form class="pt-2" @submit.prevent="upsertInvoiceReference">
                    <v-text-field
                        v-model="reference"
                        variant="outlined"
                        label="Reference (Optional)"
                        placeholder="Enter your Reference"
                        :hide-details="false"
                        :maxlength="140"
                    />
                </v-form>
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
                            color="primary"
                            variant="flat"
                            block
                            :loading="isLoading"
                            @click="upsertInvoiceReference"
                        >
                            Add
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VRow,
    VTextField,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';

const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const existingReference = computed<string>(() => billingStore.state.billingInformation?.invoiceReference ?? '');

const reference = ref<string>(existingReference.value);

async function upsertInvoiceReference(): Promise<void> {
    await withLoading(async () => {
        try {
            await billingStore.addInvoiceReference(reference.value);
            notify.success('Invoice Reference added successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error);
        }
    });
}

watch(model, value => {
    if (value) reference.value = existingReference.value;
});
</script>
