// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr @click="goToTxn">
        <th class="align-left data mobile">
            <div class="few-items">
                <p class="array-val">
                    Deposit on {{ item.formattedType }}
                </p>
                <p class="array-val">
                    <span v-if="item.type === 'storjscan'">{{ item.amount.value }}</span>
                    <span v-else>{{ item.received.value }}</span>
                </p>
                <p
                    class="array-val" :class="{
                        pending_txt: item.status === 'pending',
                        confirmed_txt: item.status === 'confirmed',
                        rejected_txt: item.status === 'rejected',
                    }"
                >
                    {{ item.formattedStatus }}
                </p>
                <p class="array-val">
                    {{ item.timestamp.toLocaleDateString() }}
                </p>
            </div>
        </th>

        <fragment>
            <th class="align-left data tablet-laptop">
                <p>{{ item.timestamp.toLocaleDateString() }}</p>
            </th>
            <th class="align-left data tablet-laptop">
                <p>Deposit on {{ item.formattedType }}</p>
                <p class="laptop">{{ item.wallet }}</p>
            </th>

            <th class="align-right data tablet-laptop">
                <p v-if="item.type === 'storjscan'">{{ item.amount.value }}</p>
                <p v-else>{{ item.received.value }}</p>
            </th>

            <th class="align-left data tablet-laptop">
                <div class="status">
                    <span
                        class="status__dot" :class="{
                            pending: item.status === 'pending',
                            confirmed: item.status === 'confirmed',
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
                >View on {{ item.formattedType }}</a>
            </th>
        </fragment>
    </tr>
</template>

<script lang="ts">
import { Prop, Component } from 'vue-property-decorator';
import { Fragment } from 'vue-fragment';

import { NativePaymentHistoryItem } from '@/types/payments';

import Resizable from '@/components/common/Resizable.vue';

// @vue/component
@Component({
    components: { Fragment },
})
export default class TokenTransactionItem extends Resizable {
    @Prop({ default: () => new NativePaymentHistoryItem() })
    private readonly item: NativePaymentHistoryItem;

    public goToTxn() {
        if (this.isMobile || this.isTablet)
            window.open(this.item.link, '_blank', 'noreferrer');
    }
}
</script>

<style scoped lang="scss">
    .pending {
        background: #ffa800;
    }

    .pending_txt {
        color: #ffa800;
    }

    .confirmed {
        background: #00ac26;
    }

    .confirmed_txt {
        color: #00ac26;
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

    @media only screen and (max-width: 425px) {

        .mobile {
            display: table-cell;
        }

        .laptop,
        .tablet-laptop {
            display: none;
        }
    }

    @media only screen and (min-width: 426px) {

        .tablet-laptop {
            display: table-cell;
        }

        .mobile {
            display: none;
        }
    }

    @media only screen and (max-width: 1024px) and (min-width: 426px) {

        .laptop {
            display: none;
        }
    }

    @media only screen and (min-width: 1024px) {

        .laptop {
            display: table-cell;
        }
    }
</style>
