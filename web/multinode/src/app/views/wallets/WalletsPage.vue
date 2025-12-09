// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wallets">
        <h1 class="wallets__title">Wallets</h1>
        <div class="wallets__content-area">
            <div class="wallets__left-area">
                <wallets-table
                    v-if="operators.length"
                    class="wallets__left-area__table"
                    :operators="operators"
                />
            </div>
            <div class="wallets__right-area">
                <info-block class="information">
                    <template #body>
                        <div class="wallets__information">
                            <h3 class="wallets__information__title">Payouts with zkSync</h3>
                            <p class="wallets__information__description">Short description how minimal threshold system works.</p>
                            <v-link uri="https://forum.storj.io/t/minimum-threshold-for-storage-node-operator-payouts/11064" label="Learn more" />
                        </div>
                    </template>
                </info-block>
            </div>
        </div>
        <div v-if="pageCount > 1" class="wallets__pagination">
            <v-pagination
                :total-page-count="pageCount"
                :preselected-current-page-number="currentPage"
                :on-page-click-callback="listPaginated"
            />
            <p class="wallets__pagination__info">Showing <strong>{{ operators.length }} of {{ totalCount }}</strong> wallets</p>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { UnauthorizedError } from '@/api';
import { useOperatorsStore } from '@/app/store/operatorsStore';
import { Operator } from '@/operators';

import InfoBlock from '@/app/components/common/InfoBlock.vue';
import VLink from '@/app/components/common/VLink.vue';
import VPagination from '@/app/components/common/VPagination.vue';
import WalletsTable from '@/app/components/wallets/tables/walletsSummary/WalletsTable.vue';

const operatorsStore = useOperatorsStore();

const pageCount = computed<number>(() => operatorsStore.state.pageCount);
const totalCount = computed<number>(() => operatorsStore.state.totalCount);
const currentPage = computed<number>(() => operatorsStore.state.currentPage);
const operators = computed<Operator[]>(() => operatorsStore.state.operators as Operator[]);

async function listPaginated(pageNumber: number): Promise<void> {
    try {
        await operatorsStore.listPaginated(pageNumber);
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }
}

onMounted(async () => {
    await listPaginated(currentPage.value);
});
</script>

<style lang="scss" scoped>
    .wallets {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);
        background-color: var(--v-background-base);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--v-header-base);
            margin-bottom: 36px;
        }

        &__content-area {
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
            width: 100%;
            min-height: 80%;
        }

        &__left-area {
            width: 75%;
            margin-right: 24px;
        }

        &__right-area {
            width: 25%;
        }

        &__information {
            font-size: 14px;
            color: var(--c-title);

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                margin-bottom: 8px;
                color: var(--v-header-base);
            }

            &__description {
                font-family: 'font_regular', sans-serif;
                margin-bottom: 16px;
                color: var(--v-text-base);
            }

            &__link {
                text-decoration: none;
                color: var(--c-primary);
            }
        }

        &__pagination {
            width: 100%;
            display: flex;
            align-items: flex-end;
            justify-content: space-between;

            &__info {
                font-family: 'font_semiBold', sans-serif;
                font-size: 16px;
                color: #74777e;

                strong {
                    color: var(--v-header-base);
                }
            }
        }
    }

    .info-block {
        padding: 20px;
        border: var(--v-border-base);

        &.information {
            background: var(--v-active-base);
        }
    }
</style>
