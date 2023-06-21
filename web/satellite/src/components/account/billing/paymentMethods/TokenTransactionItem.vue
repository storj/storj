// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr @click="goToTxn">
        <th class="align-left data mobile">
            <div class="few-items">
                <p class="array-val">
                    STORJ {{ item.formattedType }}
                </p>
                <p class="array-val">
                    <span class="amount">+ {{ item.formattedAmount }}</span>
                </p>
                <p
                    class="array-val" :class="{
                        pending_txt: item.status === 'pending',
                        confirmed_txt: item.status === 'confirmed' || item.status === 'complete',
                        rejected_txt: item.status === 'rejected',
                    }"
                >
                    {{ item.formattedStatus }}
                </p>
                <p class="array-val">
                    {{ item.timestamp.toLocaleDateString('en-US', {day:'numeric', month:'short', year:'numeric'}) }},
                    {{ item.timestamp.toLocaleTimeString('en-US', {hour:'numeric', minute:'numeric'}) }}
                </p>
            </div>
        </th>

        <th class="align-left data tablet-laptop">
            <p class="date">{{ item.timestamp.toLocaleDateString('en-US', {day:'2-digit', month:'2-digit', year:'numeric'}) }}</p>
            <p class="time">{{ item.timestamp.toLocaleTimeString('en-US', {hour:'numeric', minute:'numeric'}) }}</p>
        </th>
        <th class="align-left data tablet-laptop">
            <p>STORJ {{ item.formattedType }}</p>
            <p class="laptop">{{ item.wallet }}</p>
        </th>

        <th class="align-left data tablet-laptop">
            <p class="amount">+ {{ item.formattedAmount }}</p>
        </th>

        <th class="align-left data tablet-laptop">
            <div class="status">
                <span
                    class="status__dot" :class="{
                        pending: item.status === 'pending',
                        confirmed: item.status === 'confirmed' || item.status === 'complete',
                        rejected: item.status === 'rejected'
                    }"
                />
                <span class="status__text">{{ item.formattedStatus }}</span>
            </div>
        </th>

        <th class="align-left data laptop">
            <a
                v-if="item.link" class="download-link" target="_blank"
                rel="noopener noreferrer" :href="item.link"
            >View</a>
        </th>
    </tr>
</template>

<script setup lang="ts">
import { formatPrice } from '@/utils/strings';
import { NativePaymentHistoryItem, NativePaymentType } from '@/types/payments';
import { useResize } from '@/composables/resize';

const props = withDefaults(defineProps<{
    item: NativePaymentHistoryItem,
}>(), {
    item: () => new NativePaymentHistoryItem(),
});

const { isMobile, isTablet } = useResize();

function goToTxn() {
    if (isMobile.value || isTablet.value) {
        window.open(props.item.link, '_blank', 'noreferrer');
    }
}
</script>

<style scoped lang="scss">
    .amount {
        color: var(--c-green-5);
    }

    p,
    span {
        line-height: 20px;
    }

    .time {
        color: var(--c-grey-5);
    }

    .pending {
        background: var(--c-yellow-4);
    }

    .pending_txt {
        color: var(--c-yellow-4);
    }

    .confirmed {
        background: var(--c-green-5);
    }

    .confirmed_txt {
        color: var(--c-green-5);
    }

    .rejected {
        background: #ac1a00;
    }

    .rejected_txt {
        color: #ac1a00;
    }

    .status {
        display: flex;
        align-items: center;
        gap: 0.5rem;

        &__dot {
            height: 0.8rem;
            width: 0.8rem;
            border-radius: 100%;
        }
    }

    .download-link {
        color: #2683ff;
        text-decoration: underline !important;

        &:hover {
            color: #0059d0;
        }
    }

    .few-items {
        display: flex;
        flex-direction: column;
        justify-content: space-between;
    }

    .array-val {
        font-family: 'font_regular', sans-serif;
        font-size: 0.75rem;
        line-height: 1.25rem;

        &:first-of-type {
            font-family: 'font_bold', sans-serif;
            font-size: 0.875rem;
            margin-bottom: 3px;
        }
    }

    @media only screen and (width <= 425px) {

        .mobile {
            display: table-cell;
        }

        .laptop,
        .tablet-laptop {
            display: none;
        }
    }

    @media only screen and (width >= 426px) {

        .tablet-laptop {
            display: table-cell;
        }

        .mobile {
            display: none;
        }
    }

    @media only screen and (width <= 1024px) and (width >= 426px) {

        .laptop {
            display: none;
        }
    }

    @media only screen and (width >= 1024px) {

        .laptop {
            display: table-cell;
        }
    }
</style>
