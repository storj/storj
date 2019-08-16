// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template functional>
    <div class="sort-header-container">
        <div class="sort-header-container__item">
            <p>Bucket Name</p>
        </div>
        <div class="sort-header-container__item">
            <p>Storage Used</p>
        </div>
        <div class="sort-header-container__item">
            <p>Egress Used</p>
        </div>
        <div class="sort-header-container__item">
            <p>Objects Stored</p>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';

    @Component
    export default class SortBucketsHeader extends Vue {}
</script>

<style scoped lang="scss">
    .sort-header-container {
        display: flex;
        background: #FFFFFF;
        opacity: 0.5;
        max-height: 40px;
        align-items: center;
        
        &__item {
            width: 25%;
            margin: 0;
            cursor: pointer;
            text-align: left;
            padding-left: 26px;
            
            p {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 23px;
                color: #AFB7C1;
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
