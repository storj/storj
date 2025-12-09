// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wallet-details">
        <div class="wallet-details__header">
            <div class="wallet-details__header__left-area">
                <div class="wallet-details__header__left-area__title-area">
                    <div class="wallet-details__header__left-area__title-area__arrow" @click="redirectToWalletsSummary">
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M13.3398 0.554956C14.0797 1.2949 14.0797 2.49458 13.3398 3.23452L6.46904 10.1053H22.1053C23.1517 10.1053 24 10.9536 24 12C24 13.0464 23.1517 13.8947 22.1053 13.8947H6.46904L13.3398 20.7655C14.0797 21.5054 14.0797 22.7051 13.3398 23.445C12.5998 24.185 11.4002 24.185 10.6602 23.445L0.554956 13.3398C-0.184985 12.5998 -0.184985 11.4002 0.554956 10.6602L10.6602 0.554956C11.4002 -0.184985 12.5998 -0.184985 13.3398 0.554956Z" fill="#252A32" />
                        </svg>
                    </div>
                    <h1 class="wallet-details__header__left-area__title-area__title">Wallet</h1>
                </div>
                <p class="wallet-details__header__left-area__wallet">{{ '0xb64ef51c888972c908cfacf59b47c1afbc0ab8ac' }}</p>
                <div class="wallet-details__header__left-area__wallet-feature" :class="{ 'active': false }">
                    <template v-if="false">
                        <svg class="wallet-details__header__left-area__wallet-feature__icon" width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M15.5 8C15.5 9.98912 14.7098 11.8968 13.3033 13.3033C11.8968 14.7098 9.98912 15.5 8 15.5C6.01088 15.5 4.10322 14.7098 2.6967 13.3033C1.29018 11.8968 0.5 9.98912 0.5 8C0.5 6.01088 1.29018 4.10322 2.6967 2.6967C4.10322 1.29018 6.01088 0.5 8 0.5C9.98912 0.5 11.8968 1.29018 13.3033 2.6967C14.7098 4.10322 15.5 6.01088 15.5 8ZM11.7781 5.15937C11.7112 5.09264 11.6314 5.0401 11.5437 5.00489C11.4559 4.96968 11.362 4.95252 11.2675 4.95445C11.173 4.95637 11.0798 4.97734 10.9936 5.0161C10.9073 5.05485 10.8298 5.1106 10.7656 5.18L7.50969 9.32844L5.5475 7.36531C5.41421 7.24111 5.23792 7.1735 5.05576 7.17671C4.8736 7.17992 4.6998 7.25372 4.57098 7.38254C4.44215 7.51137 4.36836 7.68517 4.36515 7.86732C4.36193 8.04948 4.42955 8.22577 4.55375 8.35906L7.03437 10.8406C7.1012 10.9073 7.18078 10.9599 7.26836 10.9952C7.35594 11.0305 7.44973 11.0477 7.54414 11.046C7.63854 11.0442 7.73163 11.0235 7.81784 10.985C7.90405 10.9465 7.98163 10.891 8.04594 10.8219L11.7884 6.14375C11.916 6.01109 11.9865 5.8337 11.9848 5.64965C11.983 5.4656 11.9092 5.28958 11.7791 5.15937H11.7781Z" fill="#00CE7D" />
                        </svg>
                        <p class="wallet-details__header__left-area__wallet-feature__label">zkSync is opted-in</p>
                    </template>
                    <template v-else>
                        <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M16.5 9C16.5 10.9891 15.7098 12.8968 14.3033 14.3033C12.8968 15.7098 10.9891 16.5 9 16.5C7.01088 16.5 5.10322 15.7098 3.6967 14.3033C2.29018 12.8968 1.5 10.9891 1.5 9C1.5 7.01088 2.29018 5.10322 3.6967 3.6967C5.10322 2.29018 7.01088 1.5 9 1.5C10.9891 1.5 12.8968 2.29018 14.3033 3.6967C15.7098 5.10322 16.5 7.01088 16.5 9Z" fill="#586474" />
                            <rect x="5" y="8.30078" width="8" height="1.5" rx="0.75" fill="white" />
                        </svg>
                        <p class="wallet-details__header__left-area__wallet-feature__label">zkSync is opted-out</p>
                    </template>
                </div>
            </div>
            <div class="wallet-details__header__right-area">
                <p class="wallet-details__header__right-area__label">Undistributed Balance</p>
                <p class="wallet-details__header__right-area__value">{{ Currency.dollarsFromCents(25059) }}</p>
                <v-link uri="#" label="View on Etherscan" />
                <v-link uri="#" label="View on zkScan" />
            </div>
        </div>
        <div class="wallet-details__content">
            <h2 class="wallet-details__content__title">Connected Nodes</h2>
            <wallet-details-table />
        </div>
    </div>
</template>

<script setup lang="ts">
import { onBeforeMount } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { Config as RouterConfig } from '@/app/router';
import { Currency } from '@/app/utils/currency';

import VLink from '@/app/components/common/VLink.vue';
import WalletDetailsTable from '@/app/components/wallets/tables/walletDetails/WalletDetailsTable.vue';

const route = useRoute();
const router = useRouter();

function redirectToWalletsSummary(): void {
    router.push(RouterConfig.Wallets.with(RouterConfig.WalletsSummary).path);
}

onBeforeMount(() => {
    if (!route.params.address) {
        redirectToWalletsSummary();
    }
});
</script>

<style lang="scss" scoped>
    .wallet-details {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);
        color: var(--c-title);
        background-color: var(--v-background-base);

        &__header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 100%;
            padding-bottom: 32px;
            border-bottom: 1px solid var(--c-gray--light);

            &__left-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;

                &__title-area {
                    font-family: 'font_bold', sans-serif;
                    font-size: 32px;
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;

                    &__arrow {
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        width: 32px;
                        height: 32px;
                        max-width: 32px;
                        max-height: 32px;
                        cursor: pointer;
                        margin-right: 20px;
                    }

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 32px;
                        white-space: nowrap;
                        text-overflow: ellipsis;
                        position: relative;
                        overflow: hidden;
                    }
                }

                &__wallet {
                    font-family: 'font_bold', sans-serif;
                    font-size: 18px;
                    margin-top: 15px;
                }

                &__wallet-feature {
                    margin-top: 15px;
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;

                    &__icon {
                        background: white;
                        border-radius: 50%;

                        path {
                            fill: var(--c-title);
                        }
                    }

                    &__label {
                        font-family: 'font_semiBold', sans-serif;
                        font-size: 16px;
                        margin-left: 7.5px;
                        color: var(--c-label);
                    }

                    &.active {

                        svg {

                            path {
                                fill: var(--wallet-feature-opted-in);
                            }
                        }

                        p {
                            color: var(--wallet-feature-opted-in);
                        }
                    }
                }
            }

            &__right-area {
                display: flex;
                flex-direction: column;
                align-items: flex-end;

                &__label {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    color: var(--c-label);
                }

                &__value {
                    font-family: 'font_bold', sans-serif;
                    font-size: 32px;
                    margin-top: 8px;
                    margin-bottom: 20px;
                }

                & .link {
                    margin-top: 10px;
                }
            }
        }

        &__content {
            margin-top: 36px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                margin-bottom: 24px;
            }
        }
    }
</style>
