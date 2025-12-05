// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
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
                <v-card-title class="font-weight-bold">Remove Card</v-card-title>
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
                <v-card-text v-if="card.isDefault" class="py-5 px-0">
                    To remove this payment method, you will need to add a replacement first.
                    Would you like to add a new payment method now?
                </v-card-text>
                <v-card-text v-else class="py-5 px-0">This is not your default payment card.</v-card-text>

                <credit-card-item :card="card" />
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="!moreThanOneCard && card.isDefault">
                        <v-btn :prepend-icon="Plus" color="primary" variant="flat" block :loading="isLoading" @click="emit('addNew')">
                            New Payment Method
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col v-if="(card.isDefault && moreThanOneCard) || !card.isDefault">
                        <v-btn v-if="card.isDefault && moreThanOneCard" color="primary" variant="flat" block :loading="isLoading" @click="onEditDefault">
                            Set Default Card
                        </v-btn>
                        <v-btn v-if="!card.isDefault" color="error" variant="flat" block :loading="isLoading" @click="onDelete">
                            Remove
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VCardTitle,
    VCardText,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VSheet,
} from 'vuetify/components';
import { Plus, X } from 'lucide-vue-next';

import { useBillingStore } from '@/store/modules/billingStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { CreditCard } from '@/types/payments';
import { useUsersStore } from '@/store/modules/usersStore';

import CreditCardItem from '@/components/dialogs/ccActionComponents/CreditCardItem.vue';
import IconCard from '@/components/icons/IconCard.vue';

const props = defineProps<{
    card: CreditCard,
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    'editDefault': [];
    'addNew': [];
}>();

const billingStore = useBillingStore();
const usersStore = useUsersStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const moreThanOneCard = computed<boolean>(() => billingStore.state.creditCards.length > 1);

async function onDelete(): Promise<void> {
    await withLoading(async () => {
        try {
            await billingStore.removeCreditCard(props.card.id);
            notify.success('Credit card was successfully removed');
            model.value = false;
            attemptPayments();
        } catch (error) {
            error.message = `Error removing credit card. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.REMOVE_CC_MODAL);
        }
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

function onEditDefault(): void {
    emit('editDefault');
}
</script>
