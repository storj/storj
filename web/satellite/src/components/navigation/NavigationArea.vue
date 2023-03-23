// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area">
        <div ref="navigationContainer" class="navigation-area__container">
            <div class="navigation-area__container__wrap">
                <LogoIcon class="navigation-area__container__wrap__logo" @click.stop="onLogoClick" />
                <SmallLogoIcon class="navigation-area__container__wrap__small-logo" @click.stop="onLogoClick" />
                <div class="navigation-area__container__wrap__edit">
                    <ProjectSelection />
                </div>
                <div class="navigation-area__container__wrap__border" />
                <router-link
                    v-for="navItem in navigation"
                    :key="navItem.name"
                    :aria-label="navItem.name"
                    class="navigation-area__container__wrap__item-container"
                    :to="navItem.path"
                    @click.native="trackClickEvent(navItem.path)"
                >
                    <div class="navigation-area__container__wrap__item-container__left">
                        <component :is="navItem.icon" class="navigation-area__container__wrap__item-container__left__image" />
                        <p class="navigation-area__container__wrap__item-container__left__label">{{ navItem.name }}</p>
                    </div>
                </router-link>
                <div class="navigation-area__container__wrap__border" />
                <div ref="resourcesContainer" class="container-wrapper">
                    <div
                        role="button"
                        tabindex="0"
                        class="navigation-area__container__wrap__item-container"
                        :class="{ active: isResourcesDropdownShown }"
                        @keyup.enter="toggleResourcesDropdown"
                        @click.stop="toggleResourcesDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <ResourcesIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Resources</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <GuidesDropdown
                        v-if="isResourcesDropdownShown"
                        :close="closeDropdowns"
                        :y-position="resourcesDropdownYPos"
                        :x-position="resourcesDropdownXPos"
                    >
                        <ResourcesLinks />
                    </GuidesDropdown>
                </div>
                <div ref="quickStartContainer" class="container-wrapper">
                    <div
                        role="button"
                        tabindex="0"
                        class="navigation-area__container__wrap__item-container"
                        :class="{ active: isQuickStartDropdownShown }"
                        @keyup.enter="toggleQuickStartDropdown"
                        @click.stop="toggleQuickStartDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <QuickStartIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Quickstart</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <GuidesDropdown
                        v-if="isQuickStartDropdownShown"
                        :close="closeDropdowns"
                        :y-position="quickStartDropdownYPos"
                        :x-position="quickStartDropdownXPos"
                    >
                        <QuickStartLinks :close-dropdowns="closeDropdowns" />
                    </GuidesDropdown>
                </div>
            </div>
            <AccountArea />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';

import ProjectSelection from '@/components/navigation/ProjectSelection.vue';
import GuidesDropdown from '@/components/navigation/GuidesDropdown.vue';
import AccountArea from '@/components/navigation/AccountArea.vue';
import ResourcesLinks from '@/components/navigation/ResourcesLinks.vue';
import QuickStartLinks from '@/components/navigation/QuickStartLinks.vue';

import LogoIcon from '@/../static/images/logo.svg';
import SmallLogoIcon from '@/../static/images/smallLogo.svg';
import AccessGrantsIcon from '@/../static/images/navigation/accessGrants.svg';
import DashboardIcon from '@/../static/images/navigation/projectDashboard.svg';
import BucketsIcon from '@/../static/images/navigation/buckets.svg';
import UsersIcon from '@/../static/images/navigation/users.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';
import QuickStartIcon from '@/../static/images/navigation/quickStart.svg';
import ArrowIcon from '@/../static/images/navigation/arrowExpandRight.svg';

// @vue/component
@Component({
    components: {
        QuickStartLinks,
        ResourcesLinks,
        ProjectSelection,
        GuidesDropdown,
        AccountArea,
        LogoIcon,
        SmallLogoIcon,
        DashboardIcon,
        AccessGrantsIcon,
        UsersIcon,
        BucketsIcon,
        ResourcesIcon,
        QuickStartIcon,
        ArrowIcon,
    },
})
export default class NavigationArea extends Vue {
    private readonly TWENTY_PIXELS = 20;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public resourcesDropdownYPos = 0;
    public resourcesDropdownXPos = 0;
    public quickStartDropdownYPos = 0;
    public quickStartDropdownXPos = 0;
    public navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        RouteConfig.Buckets.withIcon(BucketsIcon),
        RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
        RouteConfig.Users.withIcon(UsersIcon),
    ];

    public $refs!: {
        resourcesContainer: HTMLDivElement;
        quickStartContainer: HTMLDivElement;
        navigationContainer: HTMLDivElement;
    };

    private windowWidth = window.innerWidth;

    /**
     * Mounted hook after initial render.
     * Adds scroll event listener to close dropdowns.
     */
    public mounted(): void {
        this.$refs.navigationContainer.addEventListener('scroll', this.closeDropdowns);
        window.addEventListener('resize', this.onResize);
    }

    /**
     * Mounted hook before component destroy.
     * Removes scroll event listener.
     */
    public beforeDestroy(): void {
        this.$refs.navigationContainer.removeEventListener('scroll', this.closeDropdowns);
        window.removeEventListener('resize', this.onResize);
    }

    /**
     * On screen resize handler.
     */
    public onResize(): void {
        this.windowWidth = window.innerWidth;
        this.closeDropdowns();
    }

    /**
     * Redirects to project dashboard.
     */
    public onLogoClick(): void {
        if (this.isAllProjectsDashboard) {
            this.$router.push(RouteConfig.AllProjectsDashboard.path);
            return;
        }

        if (this.$route.name === RouteConfig.ProjectDashboard.name) {
            return;
        }

        this.$router.push(RouteConfig.ProjectDashboard.path);
    }

    /**
     * Sets resources dropdown Y position depending on container's current position.
     * It is used to handle small screens.
     */
    public setResourcesDropdownYPos(): void {
        const container = this.$refs.resourcesContainer.getBoundingClientRect();
        this.resourcesDropdownYPos =  container.top + container.height / 2;
    }

    /**
     * Sets resources dropdown X position depending on container's current position.
     * It is used to handle small screens.
     */
    public setResourcesDropdownXPos(): void {
        this.resourcesDropdownXPos = this.$refs.resourcesContainer.getBoundingClientRect().width - this.TWENTY_PIXELS;
    }

    /**
     * Sets quick start dropdown Y position depending on container's current position.
     * It is used to handle small screens.
     */
    public setQuickStartDropdownYPos(): void {
        const container = this.$refs.quickStartContainer.getBoundingClientRect();
        this.quickStartDropdownYPos =  container.top + container.height / 2;
    }

    /**
     * Sets quick start dropdown X position depending on container's current position.
     * It is used to handle small screens.
     */
    public setQuickStartDropdownXPos(): void {
        this.quickStartDropdownXPos = this.$refs.quickStartContainer.getBoundingClientRect().width - this.TWENTY_PIXELS;
    }

    /**
     * Toggles resources dropdown visibility.
     */
    public toggleResourcesDropdown(): void {
        this.setResourcesDropdownYPos();
        this.setResourcesDropdownXPos();
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.RESOURCES);
    }

    /**
     * Toggles quick start dropdown visibility.
     */
    public toggleQuickStartDropdown(): void {
        this.setQuickStartDropdownYPos();
        this.setQuickStartDropdownXPos();
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.QUICK_START);
    }

    /**
     * Closes dropdowns.
     */
    public closeDropdowns(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Indicates if resources dropdown shown.
     */
    public get isResourcesDropdownShown(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.RESOURCES;
    }

    /**
     * Indicates if quick start dropdown shown.
     */
    public get isQuickStartDropdownShown(): boolean {
        return this.$store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.QUICK_START;
    }

    /**
     * Indicates if all projects dashboard should be used.
     */
    public get isAllProjectsDashboard(): boolean {
        return this.$store.state.appStateModule.isAllProjectsDashboard;
    }

    /**
     * Sends new path click event to segment.
     */
    public trackClickEvent(path: string): void {
        if (path === '/account/billing') {
            this.routeToOverview();
        } else {
            this.analytics.pageVisit(path);
        }
    }

    /**
     * Routes for new billing screens.
     */
    public routeToOverview(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
    }
}
</script>

<style scoped lang="scss">
    .navigation-svg-path {
        fill: rgb(53 64 73);
    }

    .container-wrapper {
        width: 100%;
    }

    .navigation-area {
        min-width: 280px;
        max-width: 280px;
        background-color: #fff;
        font-family: 'font_regular', sans-serif;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        border-right: 1px solid var(--c-grey-2);

        &__container {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: space-between;
            overflow-x: hidden;
            overflow-y: auto;
            width: 100%;
            height: 100%;

            &__wrap {
                display: flex;
                flex-direction: column;
                align-items: center;
                width: 100%;
                padding-top: 40px;

                &__logo {
                    cursor: pointer;
                    min-height: 37px;
                    width: 207px;
                    height: 37px;
                }

                &__small-logo {
                    display: none;
                }

                &__edit {
                    margin-top: 40px;
                    width: 100%;
                }

                &__item-container {
                    padding: 22px 32px;
                    width: 100%;
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    border: none;
                    border-left: 4px solid #fff;
                    color: var(--c-grey-6);
                    position: static;
                    cursor: pointer;
                    box-sizing: border-box;

                    &__left {
                        display: flex;
                        align-items: center;

                        &__label {
                            font-size: 14px;
                            line-height: 20px;
                            margin-left: 24px;
                        }
                    }

                    &:hover {
                        border-color: var(--c-grey-1);
                        background-color: var(--c-grey-1);
                        color: var(--c-blue-3);

                        :deep(path) {
                            fill: var(--c-blue-3);
                        }
                    }

                    &:focus {
                        outline: none;
                        border-color: var(--c-grey-1);
                        background-color: var(--c-grey-1);
                        color: var(--c-blue-3);

                        :deep(path) {
                            fill: var(--c-blue-3);
                        }
                    }
                }

                &__border {
                    margin: 8px 24px;
                    height: 1px;
                    width: calc(100% - 48px);
                    background: var(--c-grey-2);
                }
            }
        }
    }

    .router-link-active,
    .active {
        border-color: #000;
        color: var(--c-blue-6);
        font-family: 'font_bold', sans-serif;

        :deep(path) {
            fill: #000;
        }

        &:hover {
            color: var(--c-blue-3);
            border-color: var(--c-blue-3);

            :deep(path) {
                fill: var(--c-blue-3);
            }
        }
    }

    :deep(.dropdown-item) {
        display: flex;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        padding: 10px 16px;
        cursor: pointer;
        border-top: 1px solid var(--c-grey-2);
        border-bottom: 1px solid var(--c-grey-2);
    }

    :deep(.dropdown-item__icon) {
        max-width: 40px;
        min-width: 40px;
    }

    :deep(.dropdown-item__text) {
        margin-left: 10px;
    }

    :deep(.dropdown-item__text__title) {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 22px;
        color: var(--c-blue-6);
    }

    :deep(.dropdown-item__text__label) {
        font-size: 12px;
        line-height: 21px;
        color: var(--c-blue-6);
    }

    :deep(.dropdown-item:first-of-type) {
        border-radius: 8px 8px 0 0;
    }

    :deep(.dropdown-item:last-of-type) {
        border-radius: 0 0 8px 8px;
    }

    :deep(.dropdown-item:hover) {
        background-color: var(--c-grey-1);
    }

    :deep(.dropdown-item:hover h2),
    :deep(.dropdown-item:hover p) {
        color: var(--c-blue-3);
    }

    @media screen and (max-width: 1280px) {

        .navigation-area {
            min-width: unset;
            max-width: unset;

            &__container__wrap {

                &__border {
                    margin: 8px 16px;
                    width: calc(100% - 32px);
                }

                &__logo {
                    display: none;
                }

                &__item-container {
                    justify-content: center;
                    align-items: center;
                    padding: 10px 27px 10px 23px;

                    &__left {
                        flex-direction: column;

                        &__label {
                            font-family: 'font_medium', sans-serif;
                            font-size: 9px;
                            margin: 10px 0 0;
                        }
                    }

                    &__arrow {
                        display: none;
                    }
                }

                &__small-logo {
                    cursor: pointer;
                    min-height: 40px;
                    display: block;
                }
            }
        }

        .router-link-active,
        .active {
            font-family: 'font_medium', sans-serif;
        }
    }
</style>
