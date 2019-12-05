// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="limits-container">
        <div class="limits-container__item">
            <p class="limits-container__item__title">Storage available</p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">0.5 TBm</p>
                <p class="limits-container__item__values-container__total">/  1TBm</p>
            </div>
        </div>
        <div class="limits-container__item">
            <p class="limits-container__item__title">Egress available</p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">0.5 TB</p>
                <p class="limits-container__item__values-container__total">/  1 TB</p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component
    export default class ProjectLimitsArea extends Vue {

    }
</script>

<style scoped lang="scss">
    .limits-container {
        width: 100%;
        display: flex;
        justify-content: space-between;
        align-items: center;

        &__item {
            padding: 37px 28px;
            width: calc(49% - 56px);
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: flex-start;
            background-color: #fff;
            border-radius: 6px;

            &__title {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                text-align: left;
                color: #afb7c1;
                margin: 0;
            }

            &__values-container {
                display: flex;
                align-items: flex-start;
                justify-content: center;
                margin-top: 10px;

                &__remaining {
                    font-family: 'font_bold', sans-serif;
                    font-size: 36px;
                    color: #39464f;
                    margin: 0;
                }

                &__total {
                    font-family: 'font_medium', sans-serif;
                    font-size: 36px;
                    color: #afb7c1;
                    margin: 0 0 0 15px;
                }
            }
        }
    }
</style>
