// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card title="Card" class="pa-2">
        <v-card-text>
            <v-chip color="primary" size="small" variant="tonal" class="font-weight-bold mr-2 text-capitalize">{{ card.brand }}</v-chip>
            <v-chip v-if="card.isDefault" color="default" size="small" variant="tonal" class="font-weight-bold">Default</v-chip>
            <v-divider class="my-6 border-0" />
            <p>Card Number</p>
            <v-chip color="default" variant="text" class="pl-0 font-weight-bold mt-2">**** **** **** {{ card.last4 }}</v-chip>
            <v-divider class="my-6 border-0" />
            <p>Exp. Date</p>
            <v-col v-if="isEditing" class="pl-0 pb-0">
                <div class="d-flex justify-start align-end">
                    <v-number-input
                        v-model="expMonth" class="mr-1"
                        density="compact" variant="outlined"
                        :min="1" :max="12" :disabled="isLoading"
                        max-width="50" hide-details
                    />
                    <v-number-input
                        v-model="expYear" density="compact"
                        :min="currentYear" :disabled="isLoading"
                        variant="outlined" max-width="75"
                        hide-details
                    />
                </div>
            </v-col>
            <v-chip v-else :color="card.isExpired ? 'error' : 'default'" variant="text" class="pl-0 font-weight-bold mt-2">
                <v-row>
                    <v-col cols="12" class="d-flex justify-start align-center pt-2">
                        {{ card.expMonth }}/{{ card.expYear }}
                        <v-chip v-if="card.isExpired" color="error" size="small" variant="tonal" class="font-weight-bold ml-2">Expired</v-chip>
                    </v-col>
                </v-row>
            </v-chip>
            <v-divider class="my-6 border-0" />
            <v-row class="ma-0 align-center">
                <template v-if="!isEditing">
                    <v-btn
                        variant="outlined"
                        color="default"
                        class="mr-2"
                        :prepend-icon="Edit"
                        @click="isEditing = true"
                    >
                        Edit
                    </v-btn>
                    <v-btn
                        v-if="isMultipleCards"
                        variant="outlined"
                        color="default"
                        class="mr-2"
                        :prepend-icon="Star"
                        @click="isEditDefaultCCDialog = true"
                    >
                        Edit Default
                    </v-btn>
                    <v-btn variant="outlined" color="default" :prepend-icon="X" @click="isRemoveCCDialog = true">Remove</v-btn>
                </template>
                <template v-else class="ma-0 align-center">
                    <v-btn variant="outlined" color="primary" class="mr-2" :loading="isLoading" :disabled="savingDisabled" @click="saveCard">Save</v-btn>
                    <v-btn variant="outlined" color="default" :disabled="isLoading" @click="isEditing = false">Cancel</v-btn>
                </template>
            </v-row>
        </v-card-text>
    </v-card>
    <RemoveCreditCardDialog v-model="isRemoveCCDialog" :card="card" @add-new="isAddCCDialog = true" @edit-default="isEditDefaultCCDialog = true" />
    <EditDefaultCreditCardDialog v-model="isEditDefaultCCDialog" :scrim="!isRemoveCCDialog" />
    <AddCardDialog v-model="isAddCCDialog" :scrim="!isRemoveCCDialog" />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VBtn, VCard, VCardText, VChip, VCol, VDivider, VRow, VNumberInput } from 'vuetify/components';
import { X, Edit, Star } from 'lucide-vue-next';

import { CreditCard } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';

import RemoveCreditCardDialog from '@/components/dialogs/RemoveCreditCardDialog.vue';
import EditDefaultCreditCardDialog from '@/components/dialogs/EditDefaultCreditCardDialog.vue';
import AddCardDialog from '@/components/dialogs/AddCardDialog.vue';

const billingStore = useBillingStore();
const notify = useNotify();
const usersStore = useUsersStore();
const { withLoading, isLoading } = useLoading();

const props = defineProps<{
    card: CreditCard,
}>();

const currentYear = new Date().getFullYear();

const isRemoveCCDialog = ref<boolean>(false);
const isEditDefaultCCDialog = ref<boolean>(false);
const isAddCCDialog = ref<boolean>(false);
const isEditing = ref(false);
const expMonth = ref<number>(props.card.expMonth);
const expYear = ref<number>(props.card.expYear);

const savingDisabled = computed(() => {
    return expMonth.value === props.card.expMonth && expYear.value === props.card.expYear;
});

const isMultipleCards = computed<boolean>(() => billingStore.state.creditCards.length > 1);

async function saveCard() {
    await withLoading(async () => {
        try {
            await billingStore.updateCreditCard({
                cardID: props.card.id,
                expMonth: expMonth.value,
                expYear: expYear.value,
            });
            await billingStore.getCreditCards();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
            return;
        }

        isEditing.value = false;
        attemptPayments();
        notify.success('Default credit card was successfully edited');
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
</script>

<style lang="scss" scoped>
:deep(div.v-field__append-inner) {
    display: none;
}
</style>
