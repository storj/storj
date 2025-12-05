// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        scrollable
        max-width="450px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-6">
                <v-card-title class="font-weight-bold">Update Billing Address</v-card-title>
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

            <v-card-item class="px-5 py-6">
                <stripe-address-element ref="stripeAddressForm" :initial="billingAddress" />
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
                            @click="saveAddress"
                        >
                            Save
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardActions, VCardItem, VCardTitle, VCol, VDialog, VDivider, VRow } from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { BillingAddress } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';

import StripeAddressElement from '@/components/StripeAddressElement.vue';

const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const stripeAddressForm = ref<{ onSubmit(): Promise<BillingAddress> }>();

const billingAddress = computed<BillingAddress | null>(() => billingStore.state.billingInformation?.address ?? null);

function saveAddress() {
    withLoading(async () => {
        if (!stripeAddressForm.value) {
            return;
        }
        try {
            const address = await stripeAddressForm.value.onSubmit();
            await billingStore.saveBillingAddress(address);
            notify.success('Billing address saved successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error);
        }
    });
}
</script>
