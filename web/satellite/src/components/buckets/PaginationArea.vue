// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="pagination-container">
            <div class="pagination-container__pages">
                <div v-html="arrowLeft" v-on:click="prevPage" class="pagination-container__button"></div>
                <div class="pagination-container__items">
                    <span v-for="(value, index) in pages" v-bind:class="isSelected(index+1)" v-on:click="onPageClick($event, index+1)">{{index+1}}</span>
                </div>
                <div v-html="arrowRight" v-on:click="nextPage" class="pagination-container__button"></div>
            </div>
            <div class="pagination-container__counter">
                <p>Showing <span>{{firstEdge}}</span> to <span>{{lastEdge}}</span> of <span>{{totalCount}}</span> entries.</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';
    import { BUCKET_USAGE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

    @Component({
        data: function() {
            return {
                arrowLeft: EMPTY_STATE_IMAGES.ARROW_LEFT,
                arrowRight: EMPTY_STATE_IMAGES.ARROW_RIGHT,
            };
        },
        methods: {
            onPageClick: async function (event: any, page: number) {
                const response = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, page);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + response.errorMessage);
                }
            },
            isSelected: function (page: number): string {
                return page === (this as any).currentPage ? "selected" : "";
            },
            nextPage: async function() {
                if ((this as any).isLastPage) {
                    return
				}

                const response = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, (this as any).currentPage + 1);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + response.errorMessage);
                }
            },
            prevPage: async function() {
                if ((this as any).isFirstPage) {
                    return
                }

                const response = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, (this as any).currentPage - 1);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + response.errorMessage);
                }
            }
        },
        computed: {
            pages: function(): number[] {
                return new Array(this.$store.state.bucketUsageModule.page.pageCount);
            },
            currentPage: function (): number {
                return this.$store.state.bucketUsageModule.page.currentPage;
            },
            firstEdge: function (): number {
                return this.$store.state.bucketUsageModule.page.offset + 1;
            },
            lastEdge: function (): number {
                let offset = this.$store.state.bucketUsageModule.page.offset;
                let bucketsLength = this.$store.state.bucketUsageModule.page.bucketUsages.length;

                return offset + bucketsLength;
            },
            totalCount: function (): number {
                return this.$store.state.bucketUsageModule.page.totalCount;
            },
            isFirstPage: function() {
				return this.$store.state.bucketUsageModule.page.currentPage === 1;
            },
            isLastPage: function (): boolean {
                let currentPage = this.$store.state.bucketUsageModule.page.currentPage;
                let pageCount = this.$store.state.bucketUsageModule.page.pageCount;

                return currentPage === pageCount;
            }
        }
    })

    export default class PaginationArea extends Vue {}
</script>

<style scoped lang="scss">
    .pagination-container {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding-left: 25px;
        margin-top: 25px;
        
        &__pages {
            display: flex;
            align-items: center;
        }
        
        &__counter {
            
            p {
                font-family: 'font_medium';
                font-size: 16px;
                color: #AFB7C1;
            }
        }
        
        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            border: 1px solid #AFB7C1;
            border-radius: 6px;
            width: 30px;
            height: 30px;
            
            &:hover {
            
                svg {
                
                    path {
                        fill: #fff !important;
                    }
                }
            }
        }
        
        &__items {
            margin: 0 20px;
            display: flex;
            
            .selected {
                color: #2379EC;
                font-family: 'font_bold';
                
                &:after {
                    content: '';
                    display: block;
                    position: absolute;
                    bottom: -4px;
                    left: 0;
                    width: 10px;
                    height: 2px;
                    background-color: #2379EC;
                }
            }
            
            span {
                font-family: 'font_medium';
                font-size: 16px;
                margin-right: 15px;
                cursor: pointer;
                display: block;
                position: relative;
                transition: all .2s ease;
                
                &:hover {
                    color: #2379EC;
                    
                    &:after {
                        content: '';
                        display: block;
                        position: absolute;
                        bottom: -4px;
                        left: 0;
                        width: 100%;
                        height: 2px;
                        background-color: #2379EC;
                    }
                }
                
                &:last-child {
                    margin-right: 0;
                }
            }
        }
    }
</style>
