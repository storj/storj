// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <v-row>
            <v-col cols="12" lg="4">
                <v-card :loading="isLoading" title="Address" variant="outlined" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip v-if="!billingAddress" rounded color="default" variant="tonal" size="small" class="font-weight-bold">
                            No billing address added
                        </v-chip>
                        <template v-else>
                            <p>{{ billingAddress.name }}</p>
                            <p>{{ billingAddress.line1 }}</p>
                            <p>{{ billingAddress.line2 }}</p>
                            <p>{{ billingAddress.city }}</p>
                            <p>{{ billingAddress.state }}</p>
                            <p>{{ billingAddress.postalCode }}</p>
                            <p>{{ billingAddress.country.name }}</p>
                        </template>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" color="default" size="small" @click="isAddressDialogShown = true">
                            Update Address
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col v-if="!taxIDs.length" cols="12" lg="4">
                <v-card :loading="isLoading" title="Tax Information" variant="outlined" :border="true" rounded="xlg">
                    <v-card-text>
                        <v-chip rounded color="default" variant="tonal" size="small" class="font-weight-bold">
                            No tax information added
                        </v-chip>
                        <v-divider class="my-4" />
                        <v-btn variant="outlined" color="default" size="small" @click="isTaxIdDialogShown = true">
                            Add Tax ID
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
            <v-col v-for="(taxID, index) in taxIDs" v-else :key="index" cols="12" lg="4">
                <v-card :title="taxID.tax.name" variant="outlined" :border="true" rounded="xlg">
                    <v-card-text>
                        <p>{{ taxID.value }}</p>
                        <v-divider class="my-4" />
                        <v-btn :loading="isLoading" class="mr-2" variant="outlined" color="error" size="small" @click="removeTaxID(taxID.id ?? '')">
                            Remove
                        </v-btn>
                        <v-btn v-if="index === taxIDs.length - 1" color="primary" size="small" @click="isTaxIdDialogShown = true">
                            Add Tax ID
                        </v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>
    </v-container>

    <add-tax-id-dialog v-model="isTaxIdDialogShown" />
    <billing-address-dialog v-model="isAddressDialogShown" />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VCol, VContainer, VDivider, VRow } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';

import { useBillingStore } from '@/store/modules/billingStore';
import { BillingAddress, BillingInformation, TaxID } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';

import AddTaxIdDialog from '@/components/dialogs/AddTaxIdDialog.vue';
import BillingAddressDialog from '@/components/dialogs/BillingAddressDialog.vue';

const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const isTaxIdDialogShown = ref(false);
const isAddressDialogShown = ref(false);

const billingInformation = computed<BillingInformation | null>(() => billingStore.state.billingInformation);

const billingAddress = computed<BillingAddress | undefined>(() => billingInformation.value?.address);

const taxIDs = computed<TaxID[]>(() => billingInformation.value?.taxIDs ?? []);

function removeTaxID(id: string) {
    withLoading(async () => {
        try {
            await billingStore.removeTaxID(id);
        } catch (error) {
            notify.notifyError(error);
        }
    });
}

onMounted(() => {
    withLoading(async () => {
        try {
            await billingStore.getBillingInformation();
        } catch (e) {
            notify.notifyError(e);
        }
    });
});
</script>
