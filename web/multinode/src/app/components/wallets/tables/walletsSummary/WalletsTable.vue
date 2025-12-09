// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table>
        <template #head>
            <thead>
                <tr>
                    <th class="align-left" @click="sortBy('wallet')">WALLET ADDRESS{{ sortByKey === 'wallet' ? sortArrow : '' }}</th>
                    <th @click="sortBy('undistributed')">UNDISTRIBUTED{{ sortByKey === 'undistributed' ? sortArrow : '' }}</th>
                    <th class="align-left">VIEW</th>
                </tr>
            </thead>
        </template>
        <template #body>
            <tbody>
                <tr v-for="operator in sortedOperators" :key="operator.nodeId" class="table-item">
                    <th class="align-left">
                        <div class="column">
                            <p class="table-item__wallet" @click.prevent="() => redirectToWalletDetailsPage(operator.wallet)">
                                {{ operator.wallet }}
                            </p>
                            <div class="table-item__wallet-feature" :class="{ 'active': operator.areWalletFeaturesEnabled }">
                                <template v-if="operator.areWalletFeaturesEnabled">
                                    <svg class="table-item__wallet-feature__icon" width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                                        <path d="M15.5 8C15.5 9.98912 14.7098 11.8968 13.3033 13.3033C11.8968 14.7098 9.98912 15.5 8 15.5C6.01088 15.5 4.10322 14.7098 2.6967 13.3033C1.29018 11.8968 0.5 9.98912 0.5 8C0.5 6.01088 1.29018 4.10322 2.6967 2.6967C4.10322 1.29018 6.01088 0.5 8 0.5C9.98912 0.5 11.8968 1.29018 13.3033 2.6967C14.7098 4.10322 15.5 6.01088 15.5 8ZM11.7781 5.15937C11.7112 5.09264 11.6314 5.0401 11.5437 5.00489C11.4559 4.96968 11.362 4.95252 11.2675 4.95445C11.173 4.95637 11.0798 4.97734 10.9936 5.0161C10.9073 5.05485 10.8298 5.1106 10.7656 5.18L7.50969 9.32844L5.5475 7.36531C5.41421 7.24111 5.23792 7.1735 5.05576 7.17671C4.8736 7.17992 4.6998 7.25372 4.57098 7.38254C4.44215 7.51137 4.36836 7.68517 4.36515 7.86732C4.36193 8.04948 4.42955 8.22577 4.55375 8.35906L7.03437 10.8406C7.1012 10.9073 7.18078 10.9599 7.26836 10.9952C7.35594 11.0305 7.44973 11.0477 7.54414 11.046C7.63854 11.0442 7.73163 11.0235 7.81784 10.985C7.90405 10.9465 7.98163 10.891 8.04594 10.8219L11.7884 6.14375C11.916 6.01109 11.9865 5.8337 11.9848 5.64965C11.983 5.4656 11.9092 5.28958 11.7791 5.15937H11.7781Z" fill="#00CE7D" />
                                    </svg>
                                    <p class="table-item__wallet-feature__label">zkSync is opted-in</p>
                                </template>
                                <template v-else>
                                    <svg width="18" height="18" viewBox="0 0 18 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                                        <path d="M16.5 9C16.5 10.9891 15.7098 12.8968 14.3033 14.3033C12.8968 15.7098 10.9891 16.5 9 16.5C7.01088 16.5 5.10322 15.7098 3.6967 14.3033C2.29018 12.8968 1.5 10.9891 1.5 9C1.5 7.01088 2.29018 5.10322 3.6967 3.6967C5.10322 2.29018 7.01088 1.5 9 1.5C10.9891 1.5 12.8968 2.29018 14.3033 3.6967C15.7098 5.10322 16.5 7.01088 16.5 9Z" fill="#586474" />
                                        <rect x="5" y="8.30078" width="8" height="1.5" rx="0.75" fill="white" />
                                    </svg>
                                    <p class="table-item__wallet-feature__label">zkSync is opted-out</p>
                                </template>
                            </div>
                        </div>
                    </th>
                    <th>{{ Currency.dollarsFromCents(operator.undistributed) }}</th>
                    <th class="align-left">
                        <div class="column">
                            <v-link :uri="operator.etherscanLink" label="View on Etherscan" />
                            <v-link :uri="operator.zkscanLink" label="View on zkScan" />
                        </div>
                    </th>
                </tr>
            </tbody>
        </template>
    </base-table>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import { Operator } from '@/operators';
import { Currency } from '@/app/utils/currency';

import BaseTable from '@/app/components/common/BaseTable.vue';
import VLink from '@/app/components/common/VLink.vue';

const props = defineProps<{
    operators: Operator[];
}>();

const sortByKey = ref<string>('');
const sortDirection = ref<string>('asc');

const sortArrow = computed<string>(() => sortDirection.value === 'asc' ? ' ↑' : ' ↓');
const sortedOperators = computed<Operator[]>(() => {
    const key = sortByKey.value;
    const direction = sortDirection.value === 'asc' ? 1 : -1;
    if (key === '') return props.operators;
    return props.operators.slice().sort((a, b) => {
        if (a[key] < b[key]) return -direction;
        if (a[key] > b[key]) return direction;
        return 0;
    });
});

function redirectToWalletDetailsPage(_walletAddress: string): void {
    // TODO: uncomment when undistributed will be added.
    // this.$router.push({
    //     name: RouterConfig.Wallets.with(RouterConfig.WalletDetails).name,
    //     params: { address: walletAddress },
    // });
}

function sortBy(key: string): void {
    if (sortByKey.value === key) {
        if (sortDirection.value === 'asc') {
            sortDirection.value = 'desc';
        } else {
            // Disable sorting after three clicks (flow: asc -> desc -> disable -> asc -> ...)
            sortByKey.value = '';
        }
    } else {
        sortByKey.value = key;
        sortDirection.value = 'asc';
    }

    localStorage.setItem('walletSortByKey', sortByKey.value);
    localStorage.setItem('walletSortDirection', sortDirection.value);
}

onBeforeMount(() => {
    const savedSortByKey = localStorage.getItem('walletSortByKey');
    const savedSortDirection = localStorage.getItem('walletSortDirection');
    if (savedSortByKey) {
        sortByKey.value = savedSortByKey;
    }
    if (savedSortDirection) {
        sortDirection.value = savedSortDirection;
    }
});
</script>

<style lang="scss" scoped>
    .column {
        display: flex;
        flex-direction: column;
        align-items: flex-start;

        & .link:not(:first-of-type) {
            margin-top: 10px;
        }
    }

    .table-item {
        box-sizing: border-box;
        height: 89px;
        border: 1px solid var(--c-gray--light);
        cursor: pointer;

        &__wallet {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            color: var(--c-primary);
        }

        &__wallet-feature {
            margin-top: 10px;
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
                font-size: 14px;
                line-height: 17px;
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

    th {
        user-select: none; /* Diable user selecting the headers for sort selection */
    }
</style>
