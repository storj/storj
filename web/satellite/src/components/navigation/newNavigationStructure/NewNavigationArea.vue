// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="!isNavigationHidden" class="navigation-area">
        <LogoIcon class="navigation-area__logo" @click.stop="onLogoClick" />
        <div class="navigation-area__edit">
            <NewProjectSelection />
        </div>
        <router-link
            v-for="navItem in navigation"
            :key="navItem.name"
            :aria-label="navItem.name"
            class="navigation-area__item-container"
            :to="navItem.path"
        >
            <div class="navigation-area__item-container__link">
                <component :is="navItem.icon" class="navigation-area__item-container__link__icon" />
                <p class="navigation-area__item-container__link__title">{{ navItem.name }}</p>
            </div>
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NewProjectSelection from '@/components/navigation/newNavigationStructure/NewProjectSelection.vue';

import LogoIcon from '@/../static/images/logo.svg';
import AccessGrantsIcon from '@/../static/images/navigation/apiKeys.svg';
import DashboardIcon from '@/../static/images/navigation/dashboard.svg';
import ObjectsIcon from '@/../static/images/navigation/objects.svg';
import TeamIcon from '@/../static/images/navigation/team.svg';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

// @vue/component
@Component({
    components: {
        NewProjectSelection,
        LogoIcon,
        DashboardIcon,
        AccessGrantsIcon,
        TeamIcon,
        ObjectsIcon,
    },
})
export default class NewNavigationArea extends Vue {
    public navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        RouteConfig.Objects.withIcon(ObjectsIcon),
        RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
        RouteConfig.Users.withIcon(TeamIcon),
    ];

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

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
     * Indicates  if current route is onboarding tour.
     * Overviewstep needs navigation.
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
        padding: 40px 0 32px 0;
        min-width: 280px;
        max-width: 280px;
        background-color: #fff;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        overflow-y: auto;

        &__logo {
            cursor: pointer;
        }

        &__edit {
            padding: 0 20px;
            margin: 32px 0;
            width: calc(100% - 40px);
        }

        &__item-container {
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
                    fill: #fff;
                }
            }
        }
    }
</style>
