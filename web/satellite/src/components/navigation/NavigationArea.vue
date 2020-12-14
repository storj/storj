// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area" v-if="!isNavigationHidden">
        <EditProjectDropdown/>
        <router-link
            :aria-label="navItem.name"
            class="navigation-area__item-container"
            v-for="navItem in navigation"
            :key="navItem.name"
            :to="navItem.path"
        >
            <div class="navigation-area__item-container__link">
                <component :is="navItem.icon"></component>
                <h1 class="navigation-area__item-container__link__title">{{navItem.name}}</h1>
            </div>
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EditProjectDropdown from '@/components/navigation/EditProjectDropdown.vue';

import ApiKeysIcon from '@/../static/images/navigation/apiKeys.svg';
import DashboardIcon from '@/../static/images/navigation/dashboard.svg';
import TeamIcon from '@/../static/images/navigation/team.svg';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        DashboardIcon,
        ApiKeysIcon,
        TeamIcon,
        EditProjectDropdown,
    },
})
export default class NavigationArea extends Vue {
    /**
     * Array of navigation links with icons.
     */
    public readonly navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        RouteConfig.ApiKeys.withIcon(ApiKeysIcon),
        RouteConfig.Users.withIcon(TeamIcon),
    ];

    /**
     * Indicates if navigation side bar is hidden.
     */
    public get isNavigationHidden(): boolean {
        return this.isOnboardingTour || this.isCreateProjectPage;
    }

    /**
     * Indicates if current route is create project page.
     */
    private get isCreateProjectPage(): boolean {
        return this.$route.name === RouteConfig.CreateProject.name;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    private get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }
}
</script>

<style scoped lang="scss">
    .navigation-svg-path {
        fill: rgb(53, 64, 73);
    }

    .navigation-area {
        padding: 25px;
        min-width: 170px;
        max-width: 170px;
        background: #e6e9ef;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        &__item-container {
            flex: 0 0 auto;
            padding: 10px;
            width: calc(100% - 20px);
            display: flex;
            justify-content: flex-start;
            align-items: center;
            margin-bottom: 40px;
            text-decoration: none;

            &__link {
                display: flex;
                justify-content: flex-start;
                align-items: center;

                &__title {
                    font-size: 16px;
                    line-height: 23px;
                    color: #1b2533;
                    margin: 0 0 0 15px;
                }
            }

            &.router-link-active,
            &:hover {
                font-family: 'font_bold', sans-serif;
                background: #0068dc;
                border-radius: 6px;

                .navigation-area__item-container__link__title {
                    color: #fff;
                }

                .svg .navigation-svg-path:not(.white) {
                    fill: #fff !important;
                    opacity: 1;
                }
            }
        }
    }
</style>
