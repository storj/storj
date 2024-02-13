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
                <v-card-title class="font-weight-bold">Remove Credit Card</v-card-title>
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
                <v-card-text v-if="card.isDefault" class="py-5 px-0">This is your default payment card. It can't be removed.</v-card-text>
                <v-card-text v-else class="py-5 px-0">This is not your default payment card.</v-card-text>

                <credit-card-item :card="card" />
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col v-if="(card.isDefault && moreThanOneCard) || !card.isDefault">
                        <v-btn v-if="card.isDefault && moreThanOneCard" color="primary" variant="flat" block :loading="isLoading" @click="onEditDefault">
                            Edit Default
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
} from 'vuetify/components';

import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { CreditCard } from '@/types/payments';

import CreditCardItem from '@/components/dialogs/ccActionComponents/CreditCardItem.vue';

const props = defineProps<{
    card: CreditCard,
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    'editDefault': [];
}>();

const analyticsStore = useAnalyticsStore();
const billingStore = useBillingStore();
const configStore = useConfigStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const moreThanOneCard = computed<boolean>(() => billingStore.state.creditCards.length > 1);

async function onDelete(): Promise<void> {
    await withLoading(async () => {
        try {
            await billingStore.removeCreditCard(props.card.id);
            notify.success('Credit card was successfully removed');
            model.value = false;
        } catch (error) {
            error.message = `Error removing credit card. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.REMOVE_CC_MODAL);
        }
    });
}

function onEditDefault(): void {
    model.value = false;
    emit('editDefault');
}
</script>
