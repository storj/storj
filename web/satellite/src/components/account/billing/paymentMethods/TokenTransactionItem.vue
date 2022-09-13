// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container">
        <div class="divider" />
        <div class="container__row">
            <div class="container__row__item__date-container">
                <p class="container__row__item date">{{ billingItem.start.toLocaleDateString() }}</p>
                <p class="container__row__item time">{{ billingItem.start.toLocaleTimeString([], {hour: '2-digit', minute: '2-digit'}) }}</p>
            </div>
            <div class="container__row__item__description"> 
                <p class="container__row__item__description__text">CoinPayments {{ billingItem.description.includes("Deposit")? "Deposit": "Withdrawal" }}</p>
                <p class="container__row__item__description__id">{{ billingItem.id }}</p>
            </div>
            <p class="container__row__item amount">
                <b>
                    <span v-if="billingItem.type === 1">
                        ${{ billingItem.quantity.received.toFixed(2) }}
                    </span>
                    <span v-else>
                        ${{ billingItem.quantity.total.toFixed(2) }}
                    </span>     
                </b>
            </p>
            <p class="container__row__item status">
                <span :class="`container__row__item__circle-icon ${billingItem.status}`">
                    &#9679;
                </span>
                {{ billingItem.formattedStatus }}
            </p>
            <p class="container__row__item download">
                <a v-if="billingItem.link" class="download-link" target="_blank" :href="billingItem.link">View On CoinPayments</a>
            </p>
        </div>
    </div>
</template>

<script lang="ts">
import { Prop, Vue, Component } from 'vue-property-decorator';

import { PaymentsHistoryItem } from '@/types/payments';

// @vue/component
@Component
export default class TokenTransactionItem extends Vue {
    @Prop({ default: () => new PaymentsHistoryItem() })
    private readonly billingItem: PaymentsHistoryItem;
}
</script>

<style scoped lang="scss">
    .pending {
        color: #ffa800;
    }

    .confirmed {
        color: #00ac26;
    }

    .rejected {
        color: #ac1a00;
    }

    .divider {
        height: 1px;
        width: calc(100% + 30px);
        background-color: #e5e7eb;
        align-self: center;
    }

    .download-link {
        color: #2683ff;
        font-family: 'font_bold', sans-serif;
        text-decoration: underline !important;

        &:hover {
            color: #0059d0;
        }
    }

    .container {
        display: flex;
        flex-direction: column;

        &__row {
            display: flex;
            align-items: center;
            width: 100%;

            &__item {
                font-family: sans-serif;
                font-weight: 300;
                font-size: 16px;
                text-align: left;
                margin: 30px 0;

                &__description {
                    width: 35%;
                    display: flex;
                    flex-direction: column;
                    text-align: left;

                    &__text,
                    &__id {
                        font-family: 'font_medium', sans-serif;
                    }
                }

                &__date-container {
                    width: 15%;
                    display: flex;
                    flex-direction: column;
                }
            }
        }
    }

    .date {
        font-family: 'font_bold', sans-serif;
        margin: 0;
    }

    .time {
        color: #6b7280;
        margin: 0;
        font-size: 14px;
    }

    .description {
        font-family: 'font_medium', sans-serif;
        overflow: ellipse;
    }

    .status {
        width: 15%;
    }

    .amount {
        width: 15%;
    }

    .download {
        text-align: left;
        width: 20%;
    }
</style>
