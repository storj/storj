// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        class="mt-3 mb-2"
        density="compact"
        variant="tonal"
        :type="isSuccess ? 'success' : 'warning'"
    >
        <template #prepend>
            <v-icon>
                <Info v-if="isDefault" />
                <Clock v-if="isPending" />
                <CircleCheck v-if="isSuccess" />
            </v-icon>
        </template>

        <template #text>
            <p v-if="isDefault">
                <span class="font-weight-bold d-block">Remember: Only send STORJ tokens via approved networks.</span>
                <span class="text-center">
                    <template v-if="zkSyncEnabled">
                        Compatible networks: Ethereum (L1) or zkSync Era (L2)
                        <v-tooltip v-model="tooltipOpen">
                            <template #activator="{ props: activatorProps }">
                                <v-icon color="primary" v-bind="activatorProps" :icon="Info" size="16" />
                            </template>
                            STORJ zksync Era contract address:
                            <br>
                            {{ zkSyncContractAddress }}
                        </v-tooltip>
                    </template>
                    <template v-else>
                        Compatible network: Ethereum (L1)
                    </template>
                    <br>
                    Token type: ERC20 STORJ tokens only
                    <template v-if="bonusRate > 0">
                        <br>
                        You will receive a {{ bonusRate }}% bonus on your deposit
                    </template>
                </span>
            </p>

            <div v-if="isPending">
                <p class="banner__message">
                    <b>{{ stillPendingTransactions.length }} transaction{{ stillPendingTransactions.length > 1 ? 's' : '' }} pending...</b>
                    {{ totalValueCounter('confirmations', TXs.StillPending) }} of {{ totalConfirmations }} confirmations.
                    <br>
                    Expected value of {{ totalValueCounter('tokenValue', TXs.StillPending) }} STORJ tokens ~${{ totalValueCounter('usdValue', TXs.StillPending) }}.
                </p>
            </div>

            <div v-if="isSuccess" class="banner__row">
                <p class="banner__message">
                    Successful deposit of {{ totalValueCounter('tokenValue', TXs.All) }} STORJ tokens ~${{ totalValueCounter('usdValue', TXs.All) }}.
                    <template v-if="bonusReceived">
                        You received an additional bonus of {{ totalValueCounter('bonusTokens', TXs.All) }} STORJ tokens.
                    </template>
                </p>
            </div>
        </template>
    </v-alert>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { VAlert, VIcon, VTooltip } from 'vuetify/components';
import { CircleCheck, Clock, Info } from '@lucide/vue';

import { type PaymentWithConfirmations, PaymentStatus  } from '@/types/payments';
import { useConfigStore } from '@/store/modules/configStore';

const configStore = useConfigStore();

const props = defineProps<{
    isDefault: boolean
    isPending: boolean
    isSuccess: boolean
    pendingPayments: PaymentWithConfirmations[]
}>();

enum TXs {
    StillPending,
    All,
}

const tooltipOpen = ref(false);

/**
 * Returns whether deposits can also be sent via zkSync Era.
 */
const zkSyncEnabled = computed<boolean>(() => configStore.zkSyncDepositsEnabled);

/**
 * The STORJ token contract address on zkSync Era.
 */
const zkSyncContractAddress = computed((): string => {
    return configStore.state.config.zkSyncContractAddress;
});

/**
 * Returns the deposit bonus percentage from config store.
 */
const bonusRate = computed<number>(() => configStore.depositBonusRate);

/**
 * Returns whether any of the deposits earned a bonus.
 */
const bonusReceived = computed<boolean>(() => props.pendingPayments.some(p => p.bonusTokens > 0));

/**
 * Returns an array of still pending transactions to correctly display confirmations count.
 */
const stillPendingTransactions = computed((): PaymentWithConfirmations[] => {
    return props.pendingPayments.filter(p => p.status === PaymentStatus.Pending);
});

/**
 * Returns needed confirmations count for each transaction from config store.
 */
const neededConfirmations = computed((): number => {
    return configStore.state.config.neededTransactionConfirmations;
});

const totalConfirmations = computed((): number => {
    return neededConfirmations.value * stillPendingTransactions.value.length;
});

/**
 * Calculates total count of provided payment field from the list (i.e. tokenValue or bonusTokens).
 */
function totalValueCounter(field: keyof PaymentWithConfirmations, txs: TXs): string {
    let payments: PaymentWithConfirmations[];
    if (txs === TXs.StillPending) {
        payments = stillPendingTransactions.value;
    } else {
        payments = props.pendingPayments;
    }

    return payments.reduce((acc: number, curr: PaymentWithConfirmations) => {
        return acc + (curr[field] as number);
    }, 0).toLocaleString(undefined, { maximumFractionDigits: 2 });
}
</script>
