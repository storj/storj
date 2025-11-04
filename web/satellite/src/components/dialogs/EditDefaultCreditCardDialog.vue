// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :scrim
        :persistent="isLoading"
    >
        <v-card>
            <v-card-item class="pa-5 pl-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-card />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Set Default Card</v-card-title>
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
    VSheet,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { CreditCard } from '@/types/payments';
import { useUsersStore } from '@/store/modules/usersStore';

import CreditCardItem from '@/components/dialogs/ccActionComponents/CreditCardItem.vue';
import IconCard from '@/components/icons/IconCard.vue';

defineProps<{
    scrim: boolean,
}>();

const model = defineModel<boolean>({ required: true });

const billingStore = useBillingStore();
const usersStore = useUsersStore();

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

        attemptPayments();
        notify.success('Default credit card was successfully edited');
        model.value = false;
    });
}

async function attemptPayments() {
    const frozenOrWarned = usersStore.state.user.freezeStatus?.frozen ||
      usersStore.state.user.freezeStatus?.trialExpiredFrozen ||
      usersStore.state.user.freezeStatus?.warned;
    if (!frozenOrWarned) {
        return;
    }
    try {
        await billingStore.attemptPayments();
        await usersStore.getUser();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}

watch(model, () => {
    selectedCard.value = defaultCard.value?.id ?? '';
});
</script>
