// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-area">
        <div class="payments-area__top-container">
            <h1 class="payments-area__title">Payment Methods</h1>
            <VLoader v-if="ispaymentsFetching" />
            <div class="payments-area__container">
                <div
                    class="payments-area__container__token"
                >
                    <div class="payments-area__container__token__small-icon">   <StorjSmall />
                    </div>
                    <div class="payments-area__container__token__large-icon">   <StorjLarge />
                    </div>
                    <div class="payments-area__container__token__confirmation-container">
                        <p class="payments-area__container__token__confirmation-container__label">STORJ Token Deposit</p>
                        <span :class="`payments-area__container__token__confirmation-container__circle-icon ${depositStatus}`">
                            &#9679;
                        </span>
                        <span class="payments-area__container__token__confirmation-container__text">
                            <span>
                                {{ depositStatus }}
                            </span>
                        </span>
                    </div>

                    
                    <div class="payments-area__container__token__balance-container">
                        <p class="payments-area__container__token__balance-container__label">Total Balance</p>
                        <span class="payments-area__container__token__balance-amount">USD $ {{ balanceAmount }}</span>
                    </div>
                    <div class="payments-area__container__token__button-container">
                        <v-button
                            label='See transactions'
                            width="auto"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class=""
                        >
                        <v-button
                            label='Add funds'
                            font-size="13px"
                            width="auto"
                            height="30px"
                            class=""
                        >
                    </div>
                </div>
                <div
                    class="payments-area__container__cards"
                >
                    
                </div>
                <div 
                    class="payments-area__container__new-payments"
                    @click="toggleCreateModal"
                >
                    <div class="payments-area__container__new-payments__text-area">
                        <span class="payments-area__container__new-payments__text-area__plus-icon">+&nbsp;</span>
                        <span class="payments-area__container__new-payments__text-area__text">Add New Payment Method</span>
                    </div>
                </div>
            </div>
            <Addpayments2 
                v-if="showCreateCode"
                @toggleMethod="toggleCreateModal"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';

import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';

import { RouteConfig } from '@/router';

// @vue/component
@Component({
    components: {
        VLoader,
        StorjSmall,
        StorjLarge,
        VButton,
    },
})
export default class paymentsArea extends Vue {
    public depositStatus: string = 'Confirmed';
    public balanceAmount: number = 0.00;

    public testData = [{},{},{}]

    /**
     * Lifecycle hook after initial render.
     * Fetches payments.
     */
    public async mounted(): Promise<void> {
    }
}
</script>

<style scoped lang="scss">
    .active-discount {
        background: #dffff7;
        color: #00ac26;
    }

    .inactive-discount {
        background: #ffe1df;
        color: #ac1a00;
    }

    .active-status {
        background: #00ac26;
    }

    .inactive-status {
        background: #ac1a00;
    }

    .payments-area {

        &__title {
            font-family: sans-serif;
            font-size: 24px;
            margin: 20px 0;
        }

        &__container {
            display: flex;
            flex-wrap: wrap;

            &__token {
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                display: grid;
                grid-template-columns: 2fr 1fr 1fr;
                grid-template-rows: 1fr 1fr 1fr;
                margin: 0 10px 10px 0;
                padding: 20px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);
                background: #fff;
                overflow: hidden;
                &__small-icon{
                    grid-column: 1;
                    grid-row: 1;
                    height: 30px;
                    width: 40px;
                    background-color: #E6EDF7;
                    border-radius: 5px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                }
                &__large-icon{
                    grid-column: 1/3;
                    grid-row: 1/3;
                    margin: 0 0 auto 0;
                    position: relative;
                    top: -50px;
                    right: -130px;
                    z-index: 2;
                }

                &__confirmation-container {
                    grid-column: 1;
                    grid-row: 2;
                    z-index: 3;
                }
                &__balance-container {
                    grid-column: 2;
                    grid-row: 2;
                    z-index: 3;
                }
                &__button-container{
                    grid-column: 1/3;
                    grid-row: 4;
                    z-index: 3;
                }
            }

            &__new-payments {
                border: 2px dashed #929fb1;
                border-radius: 10px;
                max-width: 400px;
                width: 18vw;
                min-width: 227px;
                max-height: 222px;
                height: 10vw;
                min-height: 126px;
                padding: 18px;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;

                &__text-area {
                    display: flex;
                    align-items: center;
                    justify-content: center;

                    &__plus-icon {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 24px;
                    }

                    &__text {
                        color: #0149ff;
                        font-family: sans-serif;
                        font-size: 18px;
                        text-decoration: underline;
                    }
                }
            }
        }
    }
</style>