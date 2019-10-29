// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pagination-container">
        <div class="pagination-container__pages">
            <div class="pagination-container__button" @click="prevPage">
                <svg class="pagination-container__button__image" width="6" height="9" viewBox="0 0 6 9" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path class="pagination-svg-path" d="M2.80077e-07 4.26316L6 0L6 9L2.80077e-07 4.26316Z" fill="#354049"/>
                </svg>
            </div>
            <div class="pagination-container__items">
                <span class="pagination-container__items__page-number selected">1</span>
                <span class="pagination-container__items__page-number">2</span>
            </div>
            <div class="pagination-container__button">
                <svg class="pagination-container__button__image" width="6" height="9" viewBox="0 0 6 9" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path class="pagination-svg-path" d="M6 4.73684L0 9L1.20219e-06 -9.53674e-07L6 4.73684Z" fill="#354049"/>
                </svg>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class PaginationArea extends Vue {}
</script>

<style scoped lang="scss">
    .pagination-container {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-top: 39px;

        &__pages {
            display: flex;
            align-items: center;
        }

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            cursor: pointer;
            border: 1px solid #afb7c1;
            border-radius: 6px;
            width: 30px;
            height: 30px;

            &:hover {

                .pagination-svg-path {
                    fill: #fff !important;
                }
            }
        }

        &__items {
            margin: 0 20px;
            display: flex;

            .selected {
                color: #2379ec;
                font-family: 'font_bold', sans-serif;

                &:after {
                    content: '';
                    display: block;
                    position: absolute;
                    bottom: -4px;
                    left: 0;
                    width: 10px;
                    height: 2px;
                    background-color: #2379ec;
                }
            }

            &__page-number {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin-right: 15px;
                cursor: pointer;
                display: block;
                position: relative;
                transition: all 0.2s ease;

                &:hover {
                    color: #2379ec;

                    &:after {
                        content: '';
                        display: block;
                        position: absolute;
                        bottom: -4px;
                        left: 0;
                        width: 100%;
                        height: 2px;
                        background-color: #2379ec;
                    }
                }

                &:last-child {
                    margin-right: 0;
                }
            }
        }
    }
</style>
