// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <h1>Account</h1>
        <TabNavigation
            class="account-area-container__navigation"
            :navigation="navigation"/>
        <router-view />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import TabNavigation from '@/components/navigation/TabNavigation.vue';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        TabNavigation,
    },
})
export default class AccountArea extends Vue {
    public navigation: NavigationLink[] = [
        RouteConfig.Account.with(RouteConfig.Profile),
        RouteConfig.Account.with(RouteConfig.Billing),
        RouteConfig.Account.with(RouteConfig.PaymentMethods),
    ];
}
</script>

<style scoped lang="scss">
    .account-area-container {
        padding: 44px 55px 55px 55px;
        position: relative;
        
        &__navigation {
            position: absolute;
            right: 55px;
            top: 44px;
            z-index: 99;
            background-color: #F5F6FA;
        }
        
        h1 {
            position: absolute;
            left: 55px;
            top: 44px;
            z-index: 99;
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
    }
</style>
