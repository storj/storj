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
                <v-card-title class="font-weight-bold"> Add Tax ID </v-card-title>
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
                <v-form ref="form" v-model="formValid" class="pt-2" @submit.prevent="addTaxID">
                    <v-select
                        v-model="countryCode"
                        label="Country"
                        :rules="[RequiredRule]"
                        :items="countries"
                        :item-title="(item: TaxCountry) => item.name"
                        :item-value="(item: TaxCountry) => item.code"
                    />

                    <v-select
                        v-model="tax"
                        label="Tax ID type"
                        placeholder="Choose tax ID type"
                        :rules="[RequiredRule]"
                        :disabled="!countryCode"
                        :items="taxes"
                        :item-title="(item: Tax) => item.name"
                        :item-value="(item: Tax) => item"
                    />

                    <v-text-field
                        v-model="taxId"
                        :disabled="!tax"
                        variant="outlined"
                        :rules="[RequiredRule]"
                        label="Tax ID"
                        placeholder="Enter your Tax ID"
                        :hint="'e.g.: ' + tax?.example"
                        :hide-details="false"
                        :maxlength="50"
                        required
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
                            :disabled="!formValid"
                            :loading="isLoading"
                            @click="addTaxID"
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
import { computed, onMounted, ref, watch } from 'vue';
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
    VSelect,
    VTextField,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { Tax, TaxCountry } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { RequiredRule } from '@/types/common';

const billingStore = useBillingStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const model = defineModel<boolean>({ required: true });

const countryCode = ref<string>();
const tax = ref<Tax>();
const taxId = ref<string>();
const formValid = ref(false);

const form = ref<VForm>();

const countries = computed<TaxCountry[]>(() => billingStore.state.taxCountries);
const taxes = computed<Tax[]>(() => billingStore.state.taxes);

function addTaxID() {
    if (!formValid.value) {
        return;
    }
    withLoading(async () => {
        try {
            await billingStore.addTaxID(tax.value?.code ?? '', taxId.value ?? '');
            notify.success('Tax ID added successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error);
        }
    });
}

onMounted(async () => {
    try {
        await billingStore.getTaxCountries();
    } catch (e) {
        notify.notifyError(e);
    }
});

watch(countryCode, (code) => {
    withLoading(async () => {
        if (!code) {
            return;
        }
        tax.value = undefined;
        try {
            await billingStore.getCountryTaxes(code ?? '');
            if (taxes.value.length === 1) {
                tax.value = taxes.value[0];
            }
        } catch (e) {
            notify.notifyError(e);
        }
    });
});

watch(model, val => {
    if (!val) {
        form.value?.reset();
        countryCode.value = undefined;
        tax.value = undefined;
        taxId.value = undefined;
    }
});
</script>