// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <v-progress-linear rounded indeterminate :active="isLoading" />
        <v-select
            v-model="selectedTax"
            label="Tax ID type"
            placeholder="Choose tax ID type"
            :disabled="!countryCode"
            :items="taxes"
            :item-title="(item: Tax) => item.name"
            :item-value="(item: Tax) => item"
            hide-details
            class="mb-3"
        />

        <v-text-field
            v-model="taxID"
            :disabled="!selectedTax"
            variant="outlined"
            label="Tax ID"
            placeholder="Enter your Tax ID"
            :hint="'e.g.: ' + selectedTax?.example"
            :hide-details="false"
            :maxlength="50"
            class="custom"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VProgressLinear, VSelect, VTextField } from 'vuetify/components';

import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import type { PurchaseTax, Tax } from '@/types/payments';

const props = defineProps<{
    countryCode?: string;
}>();

const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const selectedTax = ref<Tax>();
const taxID = ref<string>();

const taxes = computed<Tax[]>(() => billingStore.state.taxes);

/**
 * Returns the selected, optional tax info, or undefined when not filled.
 * Called by the parent component.
 */
function getTax(): PurchaseTax | undefined {
    if (selectedTax.value && taxID.value) {
        return {
            type: selectedTax.value.code,
            value: taxID.value,
        };
    }
    return undefined;
}

watch(() => props.countryCode, (code) => {
    withLoading(async () => {
        selectedTax.value = undefined;
        taxID.value = undefined;
        if (!code) {
            return;
        }
        try {
            await billingStore.getCountryTaxes(code);
            if (taxes.value.length === 1) {
                selectedTax.value = taxes.value[0];
            }
        } catch (e) {
            notify.notifyError(e);
        }
    });
});

defineExpose({
    getTax,
});
</script>
