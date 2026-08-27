// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card
        title="STORJ Token"
        variant="flat"
        :border="true"
        rounded="xlg"
        height="100%"
        :loading="isLoading"
    >
        <template #append>
            <v-btn
                variant="outlined"
                color="default"
                size="small"
                :prepend-icon="History"
                @click="emit('viewHistory')"
            >
                View History
            </v-btn>
        </template>
        <v-card-text>
            <v-row justify="space-between">
                <v-col>
                    <p class="text-body-2 text-medium-emphasis">Deposit Address</p>
                    <v-row class="ma-0 mt-1 align-center">
                        <v-chip
                            v-if="balance?.wallet"
                            v-tooltip="balance.wallet"
                            variant="tonal"
                            size="small"
                            rounded="lg"
                        >
                            {{ shortAddress }}
                        </v-chip>
                        <span v-else class="text-disabled">No deposit address</span>

                        <InputCopyButton v-if="balance?.wallet" :value="balance.wallet" class="ml-2" />
                    </v-row>
                </v-col>
                <v-col align="end">
                    <p class="text-body-2 text-medium-emphasis">Token Balance</p>
                    <v-chip color="primary" variant="tonal" class="mt-1 font-weight-bold" rounded="lg">
                        {{ formattedBalance }}
                    </v-chip>
                </v-col>
            </v-row>
        </v-card-text>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VCardText, VCol, VChip, VRow } from 'vuetify/components';
import { History } from 'lucide-vue-next';

import { UserTokenBalance } from '@/api/client.gen';
import { useBillingStore } from '@/store/billing';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { formatPrice } from '@/utils/strings';

import InputCopyButton from '@/components/InputCopyButton.vue';

const props = defineProps<{
    userId: string;
}>();

const emit = defineEmits<{
    viewHistory: [];
}>();

const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const balance = ref<UserTokenBalance>();

/**
 * Returns the deposit address shortened for display.
 */
const shortAddress = computed<string>(() => {
    const address = balance.value?.wallet;
    if (!address) return '';
    return `${address.substring(0, 6)} . . . ${address.substring(address.length - 4)}`;
});

const formattedBalance = computed<string>(() => {
    if (!balance.value) return '-';
    return formatPrice(balance.value.balance);
});

async function fetchBalance(): Promise<void> {
    await withLoading(async () => {
        try {
            balance.value = await billingStore.getTokenBalance(props.userId);
        } catch (error) {
            notify.error(`Failed to get STORJ token balance. ${error.message}`);
        }
    });
}

watch(() => props.userId, fetchBalance, { immediate: true });
</script>
