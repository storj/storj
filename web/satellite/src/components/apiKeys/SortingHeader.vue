// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__name-item">
            <p>Key Name</p>
            <div class="sort-header-container__name-item__arrows">
                <span v-html="arrowUp"></span>
                <span class="selected" v-html="arrowDown"></span>
            </div>
        </div>
        <div class="sort-header-container__date-item">
            <p>Created</p>
            <div class="sort-header-container__name-item__arrows">
                <span v-html="arrowUp"></span>
                <span class="selected" v-html="arrowDown"></span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component

export default class SortApiKeysHeader extends Vue {
    public arrowUp: string = EMPTY_STATE_IMAGES.ARROW_UP;
    public arrowDown: string = EMPTY_STATE_IMAGES.ARROW_DOWN;
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        width: 100%;
        height: 40px;
        background-color: rgba(255, 255, 255, 0.3);
        margin-top: 31px;

        &__name-item,
        &__date-item {
            width: 40%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                margin-left: 26px;
                color: #AFB7C1;
            }

            &__arrows {
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
                padding-bottom: 12px;
                margin-left: 10px;

                span.selected {

                    svg {

                        path {
                            fill: #2683FF !important;
                        }
                    }
                }

                span {
                    height: 10px;
                }
            }

            &:nth-child(1) {
                margin-left: 0px;
            }
        }

        &__date-item {
            width: 60%;

            P {
                margin: 0;
            }
        }
    }
</style>