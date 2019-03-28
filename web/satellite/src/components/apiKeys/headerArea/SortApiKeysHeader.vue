// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p>Name</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span class="selected" v-html="arrowDown"></span>
            </div>
        </div>
        <div class="sort-header-container__item">
            <p>Macaroon</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span v-html="arrowDown"></span>
            </div>
        </div>
        <div class="sort-header-container__item">
            <p>Bucket</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span v-html="arrowDown"></span>
            </div>
        </div>
        <div class="sort-header-container__item">
            <p>Path</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span v-html="arrowDown"></span>
            </div>
        </div>
        <div class="sort-header-container__item">
            <p>Rights</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span v-html="arrowDown"></span>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { EMPTY_STATE_IMAGES } from '@/utils/constants/emptyStatesImages';

@Component({
    data: function() {
        return {
            arrowUp: EMPTY_STATE_IMAGES.ARROW_UP,
            arrowDown: EMPTY_STATE_IMAGES.ARROW_DOWN,
        };
    }
})

export default class SortApiKeysHeader extends Vue {
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: grid;
        grid-template-columns: 24% 29% 19% 19% 14%;
        width: 100%;
        height: 80px;
        transition: box-shadow .2s ease-out;
        padding-left: 20px;

        &__item {
            width: 20%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;
            margin-left: 6px;

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

            p {
                font-family: 'font_medium';
                font-size: 16px;
            }
            
            &:nth-child(1) {
                margin-left: 0px;
            }
        }
    }

    @media screen and (max-width: 1600px) {

        .sort-header-container {
            grid-template-columns: 25% 28% 19% 15% 14%;
            height: 65px;
        }
    }
</style>