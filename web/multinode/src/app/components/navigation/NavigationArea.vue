// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area">
        <storj-logo class="navigation-area__logo" />
        <router-link
            v-for="navItem in navigation"
            :key="navItem.name"
            :aria-label="navItem.name"
            class="navigation-area__item-container"
            :to="navItem.path"
        >
            <div class="navigation-area__item-container__link">
                <component :is="navItem.icon" />
                <p class="navigation-area__item-container__link__title">{{ navItem.name }}</p>
            </div>
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component } from 'vue-property-decorator';
import Vue, { VueConstructor } from 'vue';

import { Config as RouterConfig } from '@/app/router';

import MyNodesIcon from '@/../static/images/icons/navigation/nodes.svg';
import NotificationIcon from '@/../static/images/icons/navigation/notifications.svg';
import PayoutsIcon from '@/../static/images/icons/navigation/payouts.svg';
import ReputationIcon from '@/../static/images/icons/navigation/reputation.svg';
import TrafficIcon from '@/../static/images/icons/navigation/traffic.svg';
import StorjLogo from '@/../static/images/Logo.svg';

export class NavigationLink {
    constructor(
        public name: string,
        public path: string,
        public icon: VueConstructor<Vue>,
    ) {}
}

// @vue/component
@Component({
    components: {
        StorjLogo,
        MyNodesIcon,
        PayoutsIcon,
        ReputationIcon,
        TrafficIcon,
        NotificationIcon,
    },
})
export default class NavigationArea extends Vue {
    /**
     * Array of navigation links with icons.
     */
    public readonly navigation: NavigationLink[] = [
        new NavigationLink(RouterConfig.MyNodes.name, RouterConfig.MyNodes.path, MyNodesIcon),
        new NavigationLink(RouterConfig.Wallets.name, RouterConfig.Wallets.with(RouterConfig.WalletsSummary).path, PayoutsIcon),
        new NavigationLink(RouterConfig.Payouts.name, RouterConfig.Payouts.path, PayoutsIcon),
        new NavigationLink(RouterConfig.Bandwidth.name, RouterConfig.Bandwidth.path, TrafficIcon),
    ];
}
</script>

<style scoped lang="scss">
    .navigation-area {
        box-sizing: border-box;
        padding: 30px 24px;
        height: 100vh;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        border-right: 1px solid var(--v-border-base);
        background: var(--v-background-base);

        &__logo {
            margin-bottom: 62px;
        }

        &__item-container {
            flex: 0 0 auto;
            padding: 10px;
            width: calc(100% - 20px);
            display: flex;
            justify-content: flex-start;
            align-items: center;
            margin-bottom: 20px;
            text-decoration: none;

            ::v-deep path {
                fill: var(--v-text-base);
            }

            &__link {
                display: flex;
                justify-content: flex-start;
                align-items: center;

                &__title {
                    font-family: 'font_semiBold', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    margin: 0 0 0 15px;
                    white-space: nowrap;
                    color: var(--v-text-base);
                }
            }

            &.router-link-active,
            &:hover {
                background: var(--v-active-base);
                border-radius: 6px;

                .navigation-area__item-container__link__title {
                    color: var(--v-text-base);
                }

                ::v-deep path {
                    fill: var(--v-text-base) !important;
                    opacity: 1;
                }
            }
        }
    }
</style>
