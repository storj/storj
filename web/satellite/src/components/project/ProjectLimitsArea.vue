// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="limits-container">
        <div class="limits-container__item">
            <p class="limits-container__item__title"></p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">0.5 TBm</p>
                /
                <p class="limits-container__item__values-container__total">1TBm</p>
            </div>
        </div>
        <div class="limits-container__item">
            <p class="limits-container__item__title"></p>
            <div class="limits-container__item__values-container">
                <p class="limits-container__item__values-container__remaining">0.5 TB</p>
                /
                <p class="limits-container__item__values-container__total">1 TB</p>
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
        justify-content: center;
        align-items: center;

        &__item {
            padding: 37px 28px;
            width: calc(40% - 56px);
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            background-color: #fff;
        }
    }
</style>
