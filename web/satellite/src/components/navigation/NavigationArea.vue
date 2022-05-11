// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="!isNavigationHidden" class="navigation-area">
        <EditProjectDropdown />
        <router-link
            v-for="navItem in navigation"
            :key="navItem.name"
            :aria-label="navItem.name"
            class="navigation-area__item-container"
            :to="navItem.path"
            @click.native="trackClickEvent(navItem.name)"
        >
            <div class="navigation-area__item-container__link">
                <component :is="navItem.icon" class="navigation-area__item-container__link__icon" />
                <p class="navigation-area__item-container__link__title">{{ navItem.name }}</p>
            </div>
        </router-link>
        <div class="navigation-area__selection-wrapper">
            <ProjectSelection in-navigation="true" class="project-selection" />
            <ResourcesSelection in-navigation="true" class="resources-selection" />
            <SettingsSelection in-navigation="true" class="settings-selection" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import EditProjectDropdown from '@/components/navigation/EditProjectDropdown.vue';

import AccessGrantsIcon from '@/../static/images/navigation/apiKeys.svg';
import DashboardIcon from '@/../static/images/navigation/dashboard.svg';
import BucketsIcon from '@/../static/images/navigation/objects.svg';
import ProjectSelection from '@/components/header/projectsDropdown/ProjectSelection.vue';
import ResourcesSelection from '@/components/header/resourcesDropdown/ResourcesSelection.vue';
import SettingsSelection from '@/components/header/settingsDropdown/SettingsSelection.vue';
import TeamIcon from '@/../static/images/navigation/team.svg';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

// @vue/component
@Component({
    components: {
        DashboardIcon,
        AccessGrantsIcon,
        TeamIcon,
        EditProjectDropdown,
        BucketsIcon,
        ProjectSelection,
        ResourcesSelection,
        SettingsSelection
    },
})
export default class NavigationArea extends Vue {
    public navigation: NavigationLink[] = [];

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();


    /**
     * Lifecycle hook before initial render.
     * Sets navigation side bar list.
     */
    public beforeMount(): void {
        const value = MetaUtils.getMetaContent('file-browser-flow-disabled');
        if (value === "true") {
            this.navigation = [
                RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
                RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
                RouteConfig.Users.withIcon(TeamIcon),
            ];

            return;
        }

        this.navigation = [
            RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
            RouteConfig.Buckets.withIcon(BucketsIcon),
            RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
            RouteConfig.Users.withIcon(TeamIcon),
        ];
    }

    /**
     * Indicates if navigation side bar is hidden.
     */
    public get isNavigationHidden(): boolean {
        return this.isOnboardingTour || this.isCreateProjectPage;
    }

    /**
     * Sends new path click event to segment.
     */
    public trackClickEvent(name: string): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, name);
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
        fill: rgb(53 64 73);
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

        &__selection-wrapper {
            display: none;

            .project-selection {

                &__toggle-container {
                    background: none;

                    &__name {
                        font-family: 'font_medium', sans-serif;
                        font-size: 16px;
                        line-height: 23px;
                        color: #1b2533;
                        margin: 0 0 0 18px;
                        white-space: nowrap;
                    }
                }
            }
        }

        @media screen and (max-width: 768px) {

            .navigation-area {

                &__selection-wrapper {
                    display: block;
                }
            }
        }
    }
</style>
