// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="token">
        <div class="token__large-icon-container">
            <div class="token__large-icon">
                <StorjLarge />
            </div>
        </div>
        <div v-if="!showAddFunds" class="token__base">
            <div class="token__base__small-icon">
                <StorjSmall />
            </div>
            <div class="token__base__confirmation-container">
                <p class="token__base__confirmation-container__label">STORJ Token Deposit</p>
                <span :class="`token__base__confirmation-container__circle-icon ${billingItem.formattedStatus}`">
                    &#9679;
                </span>
                <span class="token__base__confirmation-container__text">
                    <span>
                        {{ billingItem.formattedStatus }}
                    </span>
                </span>
            </div>
            <div class="token__base__balance-container">
                <p class="token__base__balance-container__label">Total Balance</p>
                <span class="token__base__balance-container__text">
                    USD ${{ billingItem.quantity.received.toFixed(2) }}
                </span>
            </div>
            <VButton
                label="See transactions"
                width="120px"
                height="30px"
                is-transparent="true"
                font-size="13px"
                class="token__base__transaction-button"
                :on-press="toggleTransactionsTable"
            />
            <VButton
                label="Add funds"
                font-size="13px"
                width="80px"
                height="30px"
                class="token__base__funds-button"
                :on-press="toggleShowAddFunds"
            />
        </div>
        <div v-else class="token__add-funds">
            <h3 class="token__add-funds__title">
                STORJ Token
            </h3>
            <p class="token__add-funds__support-info">To deposit STORJ token and request higher limits, please contact <a target="_blank" rel="noopener noreferrer" href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212">Support</a></p>
            <VButton
                label="Back"
                width="100px"
                height="30px"
                is-transparent="true"
                font-size="13px"
                class="token__base__transaction-button"
                :on-press="toggleShowAddFunds"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { PaymentsHistoryItem } from '@/types/payments';

import VButton from '@/components/common/VButton.vue';

import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';

// @vue/component
@Component({
    components: {
        StorjSmall,
        StorjLarge,
        VButton,
    },
})
export default class BalanceTokenCard extends Vue {
    @Prop({ default: () => new PaymentsHistoryItem() })
    private readonly billingItem: PaymentsHistoryItem;

    private showAddFunds = false;

    public toggleShowAddFunds(): void {
        this.showAddFunds = !this.showAddFunds;
    }

    public toggleTransactionsTable(): void {
        this.$emit('showTransactions');
    }
}
</script>

<style scoped lang="scss">
    .Confirmed {
        color: #00ac26;
    }

    .Rejected {
        color: #ac1a00;
    }

    .Pending {
        color: #ffa800;
    }

    .token {
        border-radius: 10px;
        width: 348px;
        height: 203px;
        box-sizing: border-box;
        padding: 24px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        background: #fff;
        position: relative;
        font-family: 'font_regular', sans-serif;

        &__large-icon-container {
            position: absolute;
            top: 0;
            right: 0;
            height: 120px;
            width: 120px;
            z-index: 1;
            border-radius: 10px;
            overflow: hidden;
        }

        &__large-icon {
            position: absolute;
            top: -25px;
            right: -24px;
        }

        &__base {
            display: grid;
            grid-template-columns: 1.5fr 1fr;
            grid-template-rows: 4fr 1fr 1fr;
            overflow: hidden;
            font-family: sans-serif;
            z-index: 5;
            position: relative;
            height: 100%;
            width: 100%;

            &__small-icon {
                grid-column: 1;
                grid-row: 1;
                height: 30px;
                width: 40px;
                background-color: #e6edf7;
                border-radius: 5px;
                display: flex;
                justify-content: center;
                align-items: center;
            }

            &__confirmation-container {
                grid-column: 1;
                grid-row: 2;
                display: grid;
                grid-template-columns: 1fr 6fr;
                grid-template-rows: 1fr 1fr;

                &__label {
                    font-size: 12px;
                    font-weight: 700;
                    color: #56606d;
                    grid-column: 1/ span 2;
                    grid-row: 1;
                    margin: auto 0 0;
                }

                &__circle-icon {
                    grid-column: 1;
                    grid-row: 2;
                    margin: auto;
                }

                &__text {
                    font-size: 16px;
                    font-weight: 700;
                    grid-column: 2;
                    grid-row: 2;
                    margin: auto 0;
                }
            }

            &__balance-container {
                grid-column: 2;
                grid-row: 2;
                display: grid;
                grid-template-rows: 1fr 1fr;

                &__label {
                    font-size: 12px;
                    font-weight: 700;
                    color: #56606d;
                    grid-row: 1;
                    margin: auto 0 0;
                }

                &__text {
                    font-size: 16px;
                    font-weight: 700;
                    grid-row: 2;
                    margin: auto 0;
                }
            }

            &__transaction-button {
                grid-column: 1;
                grid-row: 4;
            }

            &__funds-button {
                grid-column: 2;
                grid-row: 4;
            }
        }

        &__confirmation-container {
            grid-column: 1;
            grid-row: 2;
            z-index: 3;
            display: grid;
            grid-template-columns: 1fr 6fr;
            grid-template-rows: 1fr 1fr;

            &__label {
                font-size: 12px;
                font-weight: 700;
                color: #56606d;
                grid-column: 1/ span 2;
                grid-row: 1;
                margin: auto 0 0;
            }

            &__circle-icon {
                grid-column: 1;
                grid-row: 2;
                margin: auto;
            }

            &__text {
                font-size: 16px;
                font-weight: 700;
                grid-column: 2;
                grid-row: 2;
                margin: auto 0;
            }
        }

        &__balance-container {
            grid-column: 2;
            grid-row: 2;
            z-index: 3;
            display: grid;
            grid-template-rows: 1fr 1fr;

            &__label {
                font-size: 12px;
                font-weight: 700;
                color: #56606d;
                grid-row: 1;
                margin: auto 0 0;
            }

            &__text {
                font-size: 16px;
                font-weight: 700;
                grid-row: 2;
                margin: auto 0;
            }
        }

        &__transaction-button {
            grid-column: 1;
            grid-row: 4;
            z-index: 3;
        }

        &__funds-button {
            grid-column: 2;
            grid-row: 4;
            z-index: 3;
        }

        &__add-funds {
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            height: 100%;
            width: 100%;

            &__title {
                font-family: 'font_bold', sans-serif;
            }

            &__support-info {
                font-size: 14px;
                line-height: 20px;
                color: #000;
                z-index: 1;

                a {
                    color: #0149ff;
                    text-decoration: underline !important;
                }
            }
        }
    }
</style>
