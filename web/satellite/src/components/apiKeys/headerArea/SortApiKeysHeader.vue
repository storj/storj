// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p>Name</p>
            <div class="sort-header-container__item__arrows">
                <span v-html="arrowUp"></span>
                <span v-html="arrowDown"></span>
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
        }
    }
})

export default class SortApiKeysHeader extends Vue {
}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: 100%;
        height: 90px;
        transition: box-shadow .2s ease-out;
        padding-left: 20px;

        &__item {
            width: 20%;
            display: flex;
            align-items: center;
            margin: 0;
            cursor: pointer;

            &__arrows {
                display: flex;
                flex-direction: column;
                justify-content: flex-start;
                padding-bottom: 12px;
                margin-left: 10px;

                span {
                    height: 10px;
                }
            }

            p {
                font-family: 'montserrat_medium';
                font-size: 16px;
            }
            
            &:nth-child(2) {
                margin-left: 58px;
            }
        }
    }
</style>