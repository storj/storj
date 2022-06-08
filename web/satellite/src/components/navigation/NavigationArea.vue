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
                    @click.native="trackClickEvent(navItem.name)"
                >
                    <div class="navigation-area__container__wrap__item-container__left">
                        <component :is="navItem.icon" class="navigation-area__container__wrap__item-container__left__image" />
                        <p class="navigation-area__container__wrap__item-container__left__label">{{ navItem.name }}</p>
                    </div>
                </router-link>
                <div class="navigation-area__container__wrap__border" />
                <div ref="resourcesContainer" class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        :class="{ active: isResourcesDropdownShown }"
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
                        <a
                            class="dropdown-item"
                            href="https://docs.storj.io/"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click.prevent="trackViewDocsEvent('https://docs.storj.io/')"
                        >
                            <DocsIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Docs</h2>
                                <p class="dropdown-item__text__label">Documentation for Storj</p>
                            </div>
                        </a>
                        <div class="dropdown-border" />
                        <a
                            class="dropdown-item"
                            href="https://forum.storj.io/"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click.prevent="trackViewForumEvent('https://forum.storj.io/')"
                        >
                            <ForumIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Forum</h2>
                                <p class="dropdown-item__text__label">Join our global community</p>
                            </div>
                        </a>
                        <div class="dropdown-border" />
                        <a
                            class="dropdown-item"
                            href="https://supportdcs.storj.io/hc/en-us"
                            target="_blank"
                            rel="noopener noreferrer"
                            @click.prevent="trackViewSupportEvent('https://supportdcs.storj.io/hc/en-us')"
                        >
                            <SupportIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Support</h2>
                                <p class="dropdown-item__text__label">Get technical support</p>
                            </div>
                        </a>
                    </GuidesDropdown>
                </div>
                <div ref="quickStartContainer" class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        :class="{ active: isQuickStartDropdownShown }"
                        @click.stop="toggleQuickStartDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <QuickStartIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Quick Start</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <GuidesDropdown
                        v-if="isQuickStartDropdownShown"
                        :close="closeDropdowns"
                        :y-position="quickStartDropdownYPos"
                        :x-position="quickStartDropdownXPos"
                    >
                        <div class="dropdown-item" aria-roledescription="create-project-route" @click.stop="navigateToNewProject">
                            <NewProjectIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">New Project</h2>
                                <p class="dropdown-item__text__label">Create a new project.</p>
                            </div>
                        </div>
                        <div class="dropdown-border" />
                        <div class="dropdown-item" aria-roledescription="create-ag-route" @click.stop="navigateToCreateAG">
                            <CreateAGIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Create an Access Grant</h2>
                                <p class="dropdown-item__text__label">Start the wizard to create a new access grant.</p>
                            </div>
                        </div>
                        <div class="dropdown-border" />
                        <div class="dropdown-item" aria-roledescription="objects-route" @click.stop="navigateToBuckets">
                            <UploadInWebIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Upload in Web</h2>
                                <p class="dropdown-item__text__label">Start uploading files in the web browser.</p>
                            </div>
                        </div>
                        <div class="dropdown-border" />
                        <div class="dropdown-item" aria-roledescription="cli-flow-route" @click.stop="navigateToCLIFlow">
                            <UploadInCLIIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Upload using CLI</h2>
                                <p class="dropdown-item__text__label">Start guide for using the Uplink CLI.</p>
                            </div>
                        </div>
                    </GuidesDropdown>
                </div>
            </div>
            <AccountArea />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelection from '@/components/navigation/ProjectSelection.vue';
import GuidesDropdown from '@/components/navigation/GuidesDropdown.vue';
import AccountArea from '@/components/navigation/AccountArea.vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';
import { APP_STATE_ACTIONS } from "@/utils/constants/actionNames";
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";

import LogoIcon from '@/../static/images/logo.svg';
import SmallLogoIcon from '@/../static/images/smallLogo.svg';
import AccessGrantsIcon from '@/../static/images/navigation/accessGrants.svg';
import DashboardIcon from '@/../static/images/navigation/projectDashboard.svg';
import BucketsIcon from '@/../static/images/navigation/buckets.svg';
import UsersIcon from '@/../static/images/navigation/users.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';
import QuickStartIcon from '@/../static/images/navigation/quickStart.svg';
import ArrowIcon from '@/../static/images/navigation/arrowExpandRight.svg';
import DocsIcon from '@/../static/images/navigation/docs.svg';
import ForumIcon from '@/../static/images/navigation/forum.svg';
import SupportIcon from '@/../static/images/navigation/support.svg';
import NewProjectIcon from '@/../static/images/navigation/newProject.svg';
import CreateAGIcon from '@/../static/images/navigation/createAccessGrant.svg';
import UploadInCLIIcon from '@/../static/images/navigation/uploadInCLI.svg';
import UploadInWebIcon from '@/../static/images/navigation/uploadInWeb.svg';

// @vue/component
@Component({
    components: {
        ProjectSelection,
        GuidesDropdown,
        AccountArea,
        LogoIcon,
        SmallLogoIcon,
        DashboardIcon,
        AccessGrantsIcon,
        UsersIcon,
        BillingIcon,
        BucketsIcon,
        ResourcesIcon,
        QuickStartIcon,
        ArrowIcon,
        DocsIcon,
        ForumIcon,
        SupportIcon,
        NewProjectIcon,
        CreateAGIcon,
        UploadInCLIIcon,
        UploadInWebIcon,
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
        RouteConfig.Account.with(RouteConfig.Billing).withIcon(BillingIcon),
    ];

    public $refs!: {
        resourcesContainer: HTMLDivElement;
        quickStartContainer: HTMLDivElement;
        navigationContainer: HTMLDivElement;
    };

    /**
     * Mounted hook after initial render.
     * Adds scroll event listener to close dropdowns.
     */
    public mounted(): void {
        this.$refs.navigationContainer.addEventListener('scroll', this.closeDropdowns)
        window.addEventListener('resize', this.closeDropdowns)
    }

    /**
     * Mounted hook before component destroy.
     * Removes scroll event listener.
     */
    public beforeDestroy(): void {
        this.$refs.navigationContainer.removeEventListener('scroll', this.closeDropdowns)
        window.removeEventListener('resize', this.closeDropdowns)
    }

    /**
     * Redirects to create project screen.
     */
    public navigateToCreateAG(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_AN_ACCESS_GRANT_CLICKED);
        this.closeDropdowns();
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).path).catch(() => {return;});
    }

    /**
     * Redirects to objects screen.
     */
    public navigateToBuckets(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_IN_WEB_CLICKED);
        this.closeDropdowns();
        this.$router.push(RouteConfig.Buckets.path).catch(() => {return;});
    }

    /**
     * Redirects to onboarding CLI flow screen.
     */
    public navigateToCLIFlow(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_USING_CLI_CLICKED);
        this.closeDropdowns();
        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_AG_NAME_STEP_BACK_ROUTE, this.$route.path);
        this.$router.push({name: RouteConfig.AGName.name});
    }

    /**
     * Redirects to create access grant screen.
     */
    public navigateToNewProject(): void {
        this.analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);
        this.closeDropdowns();
        this.$router.push(RouteConfig.CreateProject.path);
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
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
        this.setResourcesDropdownYPos()
        this.setResourcesDropdownXPos()
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_RESOURCES_DROPDOWN);
    }

    /**
     * Toggles quick start dropdown visibility.
     */
    public toggleQuickStartDropdown(): void {
        this.setQuickStartDropdownYPos()
        this.setQuickStartDropdownXPos()
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_QUICK_START_DROPDOWN);
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
        return this.$store.state.appStateModule.appState.isResourcesDropdownShown;
    }

    /**
     * Indicates if quick start dropdown shown.
     */
    public get isQuickStartDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isQuickStartDropdownShown;
    }

    /**
     * Sends "View Docs" event to segment and opens link.
     */
    public trackViewDocsEvent(link: string): void {
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
        window.open(link)
    }

    /**
     * Sends "View Forum" event to segment and opens link.
     */
    public trackViewForumEvent(link: string): void {
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
        window.open(link)
    }

    /**
     * Sends "View Support" event to segment and opens link.
     */
    public trackViewSupportEvent(link: string): void {
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
        window.open(link)
    }

    /**
     * Sends new path click event to segment.
     */
    public trackClickEvent(name: string): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, name);
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
                    border-left: 4px solid #fff;
                    color: #56606d;
                    font-weight: 500;
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
                }

                &__border {
                    margin: 8px 24px;
                    height: 1px;
                    width: calc(100% - 48px);
                    background: #ebeef1;
                }
            }
        }
    }

    .active {
        font-weight: 600;
        border-color: #0149ff;
        background-color: #f7f8fb;
        color: #0149ff;

        .navigation-area__container__wrap__item-container {

            &__left__image ::v-deep path,
            &__arrow ::v-deep path {
                fill: #0149ff;
            }
        }
    }

    .router-link-active,
    .navigation-area__container__wrap__item-container:hover {
        font-weight: 600;
        border-color: #0149ff;
        background-color: #f7f8fb;
        color: #0149ff;

        .navigation-area__container__wrap__item-container__left__image ::v-deep path {
            fill: #0149ff;
        }
    }

    .dropdown-item {
        display: flex;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        padding: 10px 24px;
        cursor: pointer;

        &__icon {
            margin-left: 15px;
            max-width: 37px;
            min-width: 37px;
        }

        &__text {
            margin-left: 24px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 22px;
                color: #091c45;
            }

            &__label {
                font-size: 12px;
                line-height: 21px;
                color: #091c45;
            }
        }
    }

    .dropdown-border {
        height: 1px;
        width: calc(100% - 48px);
        margin: 0 24px;
        background-color: #091c45;
        opacity: 0.1;
    }

    @media screen and (max-width: 1280px) {

        .navigation-area {
            min-width: unset;
            max-width: unset;

            &__container__wrap {

                &__logo {
                    display: none;
                }

                &__item-container {
                    justify-content: center;

                    &__left__label,
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
    }
</style>
