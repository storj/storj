// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="billing > 0" class="billing-overflow">
            <div class="billing-container">
                <div class="billing-container__table-holder">
                    <table class="billing-container__table">
                        <SortingHeader />
                        <BillingItem />
                    </table>
                </div>
                <PaginationArea />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import EmptyState from '@/components/common/EmptyStateArea.vue';
    import BillingItem from '@/components/account/billing/BillingItem.vue';
    import PaginationArea from '@/components/account/billing/PaginationArea.vue';
    import SortingHeader from '@/components/account/billing/SortingHeader.vue';

    @Component({
        components: {
            EmptyState,
            SortingHeader,
            BillingItem,
            PaginationArea,
        }
    })
    export default class BillingArea extends Vue {
        public billing: number = 1;
    }
</script>

<style scoped lang="scss">
    .billing-container {
        margin-top: 83px;
    
        &__table {
            margin-top: 20px;
            width: 100%;
        }
    }
    
    .table-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 20px 90px 0 40px;
        
        &:last-child {
            padding-left: 20px;
        }
    }
    
    @media screen and (max-height: 900px) {
        .billing-container__table-holder {
            overflow-y: scroll;
            height: 555px;
        
            &::-webkit-scrollbar,
            &::-webkit-scrollbar-track,
            &::-webkit-scrollbar-thumb {
                visibility: hidden;
            }
        }
    }
    
    @media screen and (max-height: 860px) {
        .billing-container__table-holder {
            height: 535px;
        }
    }
    
    @media screen and (max-height: 835px) {
        .billing-container__table-holder {
            height: 495px;
        }
    }
    
    @media screen and (max-height: 795px) {
        .billing-container__table-holder {
            height: 475px;
        }
    }
</style>
