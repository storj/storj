// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col cols="12" sm="12" md="6" lg="6" xl="4">
            <v-card :loading="isLoading" title="Address" class="pa-2">
                <v-card-text>
                    <v-chip v-if="!billingAddress" color="default" variant="tonal" size="small">
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
                    <v-divider class="my-4 border-0" />
                    <v-btn
                        variant="outlined"
                        color="default"
                        :prepend-icon="MapPin"
                        @click="isAddressDialogShown = true"
                    >
                        Update Address
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>
        <v-col v-if="!taxIDs.length" cols="12" sm="12" md="6" lg="6" xl="4">
            <v-card :loading="isLoading" title="Tax Information" class="pa-2">
                <v-card-text>
                    <v-chip color="default" variant="tonal" size="small">
                        No tax information added
                    </v-chip>
                    <v-divider class="my-4 border-0" />
                    <v-btn variant="outlined" color="default" :prepend-icon="Plus" @click="isTaxIdDialogShown = true">
                        Add Tax ID
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>
        <v-col v-for="(taxID, index) in taxIDs" v-else :key="index" cols="12" sm="12" md="6" lg="6" xl="4">
            <v-card :loading="isLoading" :title="taxID.tax.name" class="pa-2">
                <v-card-text>
                    <p>{{ taxID.value }}</p>
                    <v-divider class="my-4 border-0" />
                    <v-btn :loading="isLoading" class="mr-2" variant="outlined" color="error" :prepend-icon="X" @click="removeTaxID(taxID.id ?? '')">
                        Remove
                    </v-btn>
                    <v-btn v-if="index === taxIDs.length - 1" color="primary" :prepend-icon="Plus" @click="isTaxIdDialogShown = true">
                        Add Tax ID
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>
        <v-col cols="12" sm="12" md="6" lg="6" xl="4">
            <v-card :loading="isLoading" title="Invoice Reference" class="pa-2">
                <v-card-text>
                    <v-chip v-if="!invoiceReference" color="default" variant="tonal" size="small">
                        No invoice reference added
                    </v-chip>
                    <p v-else>{{ invoiceReference }}</p>
                    <v-divider class="my-4 border-0" />
                    <v-btn variant="outlined" color="default" :prepend-icon="ReceiptText" @click="isInvoiceReferenceDialogShown = true">
                        {{ invoiceReference ? 'Update' : 'Add' }} Invoice Reference
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>
        <v-col cols="12" sm="12" md="6" lg="6" xl="4">
            <v-card title="Add Invoice Recipients" class="pa-2">
                <v-card-text>
                    <p>Add email addresses to automatically receive invoices.</p>
                    <v-divider class="my-4 border-0" />
                    <v-btn link :href="requestURL" target="_blank" rel="noopener noreferrer" variant="outlined" color="default">
                        Create Support Ticket
                        <template #append>
                            <v-icon :icon="ExternalLink" right />
                        </template>
                    </v-btn>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <add-tax-id-dialog v-model="isTaxIdDialogShown" />
    <billing-address-dialog v-model="isAddressDialogShown" />
    <add-invoice-reference-dialog v-model="isInvoiceReferenceDialogShown" />
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VChip, VCol, VDivider, VRow, VIcon } from 'vuetify/components';
import { ExternalLink, Plus, X, MapPin, ReceiptText } from 'lucide-vue-next';
import { computed, onMounted, ref } from 'vue';

import { useBillingStore } from '@/store/modules/billingStore';
import { BillingAddress, BillingInformation, TaxID } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useConfigStore } from '@/store/modules/configStore';

import AddTaxIdDialog from '@/components/dialogs/AddTaxIdDialog.vue';
import BillingAddressDialog from '@/components/dialogs/BillingAddressDialog.vue';
import AddInvoiceReferenceDialog from '@/components/dialogs/AddInvoiceReferenceDialog.vue';

const billingStore = useBillingStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const isTaxIdDialogShown = ref(false);
const isAddressDialogShown = ref(false);
const isInvoiceReferenceDialogShown = ref(false);

const requestURL = computed<string>(() => configStore.state.config.generalRequestURL);

const billingInformation = computed<BillingInformation | null>(() => billingStore.state.billingInformation);

const billingAddress = computed<BillingAddress | undefined>(() => billingInformation.value?.address);

const taxIDs = computed<TaxID[]>(() => billingInformation.value?.taxIDs ?? []);

const invoiceReference = computed<string>(() => billingInformation.value?.invoiceReference ?? '');

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
