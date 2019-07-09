// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div v-if="billing > 0" class="billing-overflow">
            <div class="billing-header">
                <p>Billing</p>
            </div>
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
    import BillingItem from '@/components/project/billing/BillingItem.vue';
    import PaginationArea from '@/components/project/billing/PaginationArea.vue';
    import SortingHeader from '@/components/project/billing/SortingHeader.vue';

    @Component({
        data: function () {
            return {
                billing: 1,
            };
        },
        components: {
            EmptyState,
            SortingHeader,
            BillingItem,
            PaginationArea,
        }
    })
    export default class BillingArea extends Vue {}
</script>

<style scoped lang="scss">
    .billing-header {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        
        p {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #384B65;
            margin-right: 50px;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
    }
    
    .billing-container {
        
        &__table-holder {
            margin-top: 20px;
        }
        
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
