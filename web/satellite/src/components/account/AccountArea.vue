// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <router-view/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

/**
 * AccountArea is a container for all account related routes.
 */
@Component
export default class AccountArea extends Vue {}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar,
    ::-webkit-scrollbar-track,
    ::-webkit-scrollbar-thumb {
        width: 4px;
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-track-piece {
        background-color: #f5f6fa;
    }

    ::-webkit-scrollbar-thumb {
        background-color: #afb7c1;
    }

    .account-area-container {
        padding: 0 30px;
        height: auto;
    }
</style>
