// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="wallets">
        <h1 class="wallets__title">Wallets</h1>
        <div class="wallets__content-area">
            <div class="wallets__left-area">
                <wallets-table
                    class="wallets__left-area__table"
                />
            </div>
            <div class="wallets__right-area">
                <info-block class="information">
                    <div class="wallets__information" slot="body">
                        <h3 class="wallets__information__title">Payouts with zkSync</h3>
                        <p class="wallets__information__description">Short description how minimal threshold system works.</p>
                        <v-link uri="#" label="Learn more" />
                    </div>
                </info-block>
            </div>
        </div>
        <div class="wallets__pagination">
            <v-pagination :total-page-count="10" />
            <p class="wallets__pagination__info">Showing <strong>{{ 6 }} of {{ 200 }}</strong> wallets</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import InfoBlock from '@/app/components/common/InfoBlock.vue';
import VLink from '@/app/components/common/VLink.vue';
import VPagination from '@/app/components/common/VPagination.vue';
import WalletsTable from '@/app/components/wallets/tables/walletsSummary/WalletsTable.vue';

import { UnauthorizedError } from '@/api';

@Component({
    components: {
        VPagination,
        VLink,
        InfoBlock,
        WalletsTable,
    },
})
export default class WalletsPage extends Vue {
    public mounted(): void {
        try {
            // api call here
        } catch (error) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            // TODO: notify error
        }
    }
}
</script>

<style lang="scss" scoped>
    .wallets {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--c-title);
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
            }

            &__description {
                font-family: 'font_regular', sans-serif;
                margin-bottom: 16px;
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
                    color: var(--c-title);
                }
            }
        }
    }

    .info-block {
        padding: 20px;

        &.information {
            background: #f8f8f9;
        }
    }
</style>
