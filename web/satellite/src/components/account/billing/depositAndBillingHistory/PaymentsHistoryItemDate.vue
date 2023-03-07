// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="countdown-container">
        <div v-if="isExpired">{{ expireDate }}</div>
        <div v-else class="row">
            <p>Expires in </p>
            <p class="digit margin">{{ expirationTimer.hours | leadingZero }}</p>
            <p>:</p>
            <p class="digit">{{ expirationTimer.minutes | leadingZero }}</p>
            <p>:</p>
            <p class="digit">{{ expirationTimer.seconds | leadingZero }}</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import { PaymentsHistoryItem, PaymentsHistoryItemStatus, PaymentsHistoryItemType } from '@/types/payments';

const props = withDefaults(defineProps<{
    billingItem?: PaymentsHistoryItem,
}>(), {
    billingItem: () => new PaymentsHistoryItem(),
});

const { end, start, type, status } = props.billingItem;

const nowInSeconds = ref<number>(Math.trunc(new Date().getTime() / 1000));
const expirationTimeInSeconds = ref<number>(0);
const intervalID = ref<ReturnType<typeof setInterval>>();

/**
 * indicates if billing item is expired.
 */
const isExpired = ref<boolean>(false);

/**
 * String representation of creation date.
 */
const expireDate = computed((): string => {
    return start.toLocaleString('default', { month: 'long', day: '2-digit', year: 'numeric' });
});

/**
 * Seconds count for expiration timer.
 */
const expirationTimer = computed((): { seconds: number, minutes: number, hours: number } => {
    return {
        seconds: (expirationTimeInSeconds.value - nowInSeconds.value) % 60,
        minutes: Math.trunc((expirationTimeInSeconds.value - nowInSeconds.value) / 60) % 60,
        hours: Math.trunc((expirationTimeInSeconds.value - nowInSeconds.value) / 3600) % 24,
    };
});

/**
 * Indicates if transaction status is completed, paid or cancelled.
 */
const isTransactionCompleted = computed((): boolean => {
    return status !== PaymentsHistoryItemStatus.Pending;
});

/**
 * Starts expiration timer if item is not expired.
 */
function ready(): void {
    intervalID.value = setInterval(() => {
        if ((expirationTimeInSeconds.value - nowInSeconds.value) < 0 || isTransactionCompleted.value) {
            isExpired.value = true;
            intervalID.value && clearInterval(intervalID.value);

            return;
        }

        nowInSeconds.value = Math.trunc(new Date().getTime() / 1000);
    }, 1000);
}

onBeforeMount(() => {
    expirationTimeInSeconds.value = Math.trunc(new Date(end).getTime() / 1000);
    isExpired.value = (expirationTimeInSeconds.value - nowInSeconds.value) < 0;

    if (type === PaymentsHistoryItemType.Transaction) {
        isExpired.value = isTransactionCompleted.value;
    }

    ready();
});
</script>

<style scoped lang="scss">
    .digit {
        font-family: 'font_bold', sans-serif;
    }

    .margin {
        margin-left: 5px;
    }

    .row {
        display: flex;
        align-items: center;
        justify-content: flex-start;
    }
</style>
