// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <h1 class="account-area-container__title">Account</h1>
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
            z-index: 99;
            background-color: #F5F6FA;
        }
        
        &__title {
            position: absolute;
            left: 55px;
            z-index: 99;
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            color: #354049;
            margin-block-start: 0.5em;
            margin-block-end: 0.5em;
        }
    }

    @media screen and (max-width: 1024px) {
        .account-area-container {
            padding: 44px 40px 55px 40px;

            &__navigation {
                right: 40px;
            }

            &__title {
                left: 40px;
            }
        }
    }
</style>
