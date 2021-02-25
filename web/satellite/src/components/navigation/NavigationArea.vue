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
                <component class="navigation-area__item-container__link__icon" :is="navItem.icon"/>
                <p class="navigation-area__item-container__link__title">{{navItem.name}}</p>
            </div>
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EditProjectDropdown from '@/components/navigation/EditProjectDropdown.vue';

import AccessGrantsIcon from '@/../static/images/navigation/apiKeys.svg';
import DashboardIcon from '@/../static/images/navigation/dashboard.svg';
import ObjectsIcon from '@/../static/images/navigation/objects.svg';
import TeamIcon from '@/../static/images/navigation/team.svg';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        DashboardIcon,
        AccessGrantsIcon,
        TeamIcon,
        EditProjectDropdown,
        ObjectsIcon,
    },
})
export default class NavigationArea extends Vue {
    /**
     * Array of navigation links with icons.
     */
    public readonly navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        // TODO: enable when the flow will be finished
        // RouteConfig.Objects.withIcon(ObjectsIcon),
        RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
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
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
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

                &__icon {
                    min-width: 24px;
                }

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #1b2533;
                    margin: 0 0 0 18px;
                    white-space: nowrap;
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
