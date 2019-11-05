// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component
export default class AccountArea extends Vue {}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 4px;
        margin-top: 55px;
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-track-piece {
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-thumb {
        background-color: #afb7c1;
    }

    .account-area-container {
        padding: 0 39px 0 55px;
        margin-right: 16px;
        position: relative;
        overflow-y: scroll;
        height: 90vh;

        &__navigation {
            position: absolute;
            right: 55px;
            z-index: 99;
            background-color: #f5f6fa;
        }
    }

    @media screen and (max-width: 1024px) {

        .account-area-container {
            padding: 44px 40px 55px 40px;

            &__navigation {
                right: 40px;
            }
        }
    }
</style>
