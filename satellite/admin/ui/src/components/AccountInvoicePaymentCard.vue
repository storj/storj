// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card
        title="Invoice Payment"
        variant="flat"
        :border="true"
        rounded="xlg"
        height="100%"
        :loading="isLoading"
    >
        <template v-if="canSet" #append>
            <v-btn
                variant="outlined"
                color="default"
                size="small"
                :prepend-icon="Pencil"
                :disabled="!entitlements"
                @click="dialogEnabled = true"
            >
                Update
            </v-btn>
        </template>
        <v-card-text>
            <p class="text-body-2 text-medium-emphasis">Payment Methods</p>
            <div class="d-flex flex-wrap ga-1 mt-1">
                <v-chip
                    v-for="method in paymentMethods"
                    :key="method"
                    variant="tonal"
                    size="small"
                    rounded="lg"
                >
                    {{ method }}
                </v-chip>
                <span v-if="!paymentMethods.length" class="text-disabled">Stripe default</span>
            </div>

            <p class="text-body-2 text-medium-emphasis mt-3">Collection</p>
            <p class="mt-1">{{ collectionLabel }}</p>
        </v-card-text>
    </v-card>

    <AccountInvoicePaymentDialog
        v-if="canSet && entitlements"
        v-model="dialogEnabled"
        :user-id="userId"
        :entitlements="entitlements"
        @updated="value => entitlements = value"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardText, VChip } from 'vuetify/components';
import { Pencil } from 'lucide-vue-next';

import { UserEntitlements } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { useBillingStore } from '@/store/billing';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { CollectionMethod } from '@/types/billing';

import AccountInvoicePaymentDialog from '@/components/AccountInvoicePaymentDialog.vue';

const props = defineProps<{
    userId: string;
}>();

const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const entitlements = ref<UserEntitlements>();
const dialogEnabled = ref<boolean>(false);

const canSet = computed<boolean>(() => appStore.state.settings.admin.features.account.setEntitlements);

const paymentMethods = computed<string[]>(() => entitlements.value?.invoicePaymentMethods ?? []);

const collectionLabel = computed<string>(() => {
    if (!entitlements.value) return '-';
    switch (entitlements.value.invoiceCollectionMethod) {
    case CollectionMethod.SendInvoice:
        if (!entitlements.value.invoiceDaysUntilDue) return 'Send invoice, due immediately';
        return `Send invoice, due in ${entitlements.value.invoiceDaysUntilDue} days`;
    case CollectionMethod.ChargeAutomatically:
        return 'Charge automatically';
    default:
        return 'Stripe default';
    }
});

async function fetchEntitlements(): Promise<void> {
    await withLoading(async () => {
        try {
            entitlements.value = await billingStore.getEntitlements(props.userId);
        } catch (error) {
            notify.error(`Failed to get invoice payment settings. ${error.message}`);
        }
    });
}

watch(() => props.userId, fetchEntitlements, { immediate: true });
</script>
