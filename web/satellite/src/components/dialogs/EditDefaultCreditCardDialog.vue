// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="320px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-6">
                <v-card-title class="font-weight-bold">Edit Default Credit Card</v-card-title>
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

            <v-card-item class="px-6 py-0">
                <v-radio-group v-model="selectedCard" hide-details>
                    <credit-card-item v-if="defaultCard" class="mt-6" selectable :card="defaultCard" />
                    <credit-card-item v-for="cc in nonDefaultCards" :key="cc.id" selectable :card="cc" />
                </v-radio-group>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="primary" variant="flat" block :loading="isLoading" @click="onMakeDefault">
                            Set Default
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
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VRadioGroup,
} from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { CreditCard } from '@/types/payments';

import CreditCardItem from '@/components/dialogs/ccActionComponents/CreditCardItem.vue';

const model = defineModel<boolean>({ required: true });

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const defaultCard = computed<CreditCard | undefined>(() => {
    return billingStore.state.creditCards.find(c => c.isDefault);
});

const selectedCard = ref<string>(defaultCard.value?.id ?? '');

const nonDefaultCards = computed<CreditCard[]>(() => {
    return billingStore.state.creditCards.filter(c => !c.isDefault);
});

async function onMakeDefault(): Promise<void> {
    await withLoading(async () => {
        if (!selectedCard.value) {
            notify.error('Please select credit card from the list', AnalyticsErrorEventSource.EDIT_DEFAULT_CC_MODAL);
            return;
        }

        if (selectedCard.value !== defaultCard.value?.id) {
            try {
                await billingStore.makeCardDefault(selectedCard.value);
            } catch (error) {
                error.message = `Error making credit card default. ${error.message}`;
                notify.notifyError(error, AnalyticsErrorEventSource.EDIT_DEFAULT_CC_MODAL);
                return;
            }
        }

        notify.success('Default credit card was successfully edited');
        model.value = false;
    });
}

watch(() => props.modelValue, () => {
    selectedCard.value = defaultCard.value?.id ?? '';
});
</script>
