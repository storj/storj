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
                        <p class="navigation-area__container__wrap__item-container__left__label">{{ isSmallWindow ? navItem.name.split(' ').pop(): navItem.name }}</p>
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
                        <div class="dropdown-item" aria-roledescription="create-project-route" @click.stop="navigateToNewProject">
                            <NewProjectIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">New Project</h2>
                                <p class="dropdown-item__text__label">Create a new project.</p>
                            </div>
                        </div>
                        <div class="dropdown-item" aria-roledescription="create-ag-route" @click.stop="navigateToCreateAG">
                            <CreateAGIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Create an Access Grant</h2>
                                <p class="dropdown-item__text__label">Start the wizard to create a new access grant.</p>
                            </div>
                        </div>
                        <div class="dropdown-item" aria-roledescription="objects-route" @click.stop="navigateToBuckets">
                            <UploadInWebIcon class="dropdown-item__icon" />
                            <div class="dropdown-item__text">
                                <h2 class="dropdown-item__text__title">Upload in Web</h2>
                                <p class="dropdown-item__text__label">Start uploading files in the web browser.</p>
                            </div>
                        </div>
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
import { User } from "@/types/users";

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

    private windowWidth = window.innerWidth;

    /**
     * Mounted hook after initial render.
     * Adds scroll event listener to close dropdowns.
     */
    public mounted(): void {
        this.$refs.navigationContainer.addEventListener('scroll', this.closeDropdowns)
        window.addEventListener('resize', this.onResize)
    }

    /**
     * Mounted hook before component destroy.
     * Removes scroll event listener.
     */
    public beforeDestroy(): void {
        this.$refs.navigationContainer.removeEventListener('scroll', this.closeDropdowns)
        window.removeEventListener('resize', this.onResize)
    }

    /**
     * On screen resize handler.
     */
    public onResize(): void {
        this.windowWidth = window.innerWidth;
        this.closeDropdowns();
    }

    /**
     * Redirects to create project screen.
     */
    public navigateToCreateAG(): void {
        this.analytics.eventTriggered(AnalyticsEvent.CREATE_AN_ACCESS_GRANT_CLICKED);
        this.closeDropdowns();
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);
        this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).path).catch(() => {return;});
    }

    /**
     * Redirects to objects screen.
     */
    public navigateToBuckets(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_IN_WEB_CLICKED);
        this.closeDropdowns();
        this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
        this.$router.push(RouteConfig.Buckets.path).catch(() => {return;});
    }

    /**
     * Redirects to onboarding CLI flow screen.
     */
    public navigateToCLIFlow(): void {
        this.analytics.eventTriggered(AnalyticsEvent.UPLOAD_USING_CLI_CLICKED);
        this.closeDropdowns();
        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_AG_NAME_STEP_BACK_ROUTE, this.$route.path);
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
        this.$router.push({name: RouteConfig.AGName.name});
    }

    /**
     * Redirects to create access grant screen.
     */
    public navigateToNewProject(): void {
        if (this.$route.name !== RouteConfig.CreateProject.name) {
            this.analytics.eventTriggered(AnalyticsEvent.NEW_PROJECT_CLICKED);

            const user: User = this.$store.getters.user;
            const ownProjectsCount: number = this.$store.getters.projectsCount;

            if (!user.paidTier && user.projectLimit === ownProjectsCount) {
                this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PROMPT_POPUP);
            } else {
                this.analytics.pageVisit(RouteConfig.CreateProject.path);
                this.$router.push(RouteConfig.CreateProject.path);
            }
        }

        this.closeDropdowns();
    }

    /**
     * Redirects to project dashboard.
     */
    public onLogoClick(): void {
        if (this.$route.name === RouteConfig.ProjectDashboard.name || this.$route.name === RouteConfig.NewProjectDashboard.name) {
            return;
        }

        if (this.isNewProjectDashboard) {
            this.$router.push(RouteConfig.NewProjectDashboard.path);

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
     * Indicates if new project dashboard should be used.
     */
    public get isNewProjectDashboard(): boolean {
        return this.$store.state.appStateModule.isNewProjectDashboard;
    }

    /**
     * Indicates if window is less or equal of 1280px.
     */
    public get isSmallWindow(): boolean {
        return this.windowWidth <= 1280;
    }

    /**
     * Sends "View Docs" event to segment and opens link.
     */
    public trackViewDocsEvent(link: string): void {
        this.analytics.pageVisit(link);
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
        window.open(link)
    }

    /**
     * Sends "View Forum" event to segment and opens link.
     */
    public trackViewForumEvent(link: string): void {
        this.analytics.pageVisit(link);
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_FORUM_CLICKED);
        window.open(link)
    }

    /**
     * Sends "View Support" event to segment and opens link.
     */
    public trackViewSupportEvent(link: string): void {
        this.analytics.pageVisit(link);
        this.analytics.eventTriggered(AnalyticsEvent.VIEW_SUPPORT_CLICKED);
        window.open(link)
    }

    /**
     * Sends new path click event to segment.
     */
    public trackClickEvent(path: string): void {
        this.analytics.pageVisit(path);
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
                        border-color: #fafafb;
                        background-color: #fafafb;
                        color: #0149ff;

                        ::v-deep path {
                            fill: #0149ff;
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

    .router-link-active,
    .active {
        border-color: #000;
        color: #091c45;
        font-family: 'font_bold', sans-serif;

        ::v-deep path {
            fill: #000;
        }

        &:hover {
            color: #0149ff;
            border-color: #0149ff;

            ::v-deep path {
                fill: #0149ff;
            }
        }
    }

    .dropdown-item {
        display: flex;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        padding: 10px 16px;
        cursor: pointer;
        border-top: 1px solid #ebeef1;
        border-bottom: 1px solid #ebeef1;

        &__icon {
            max-width: 40px;
            min-width: 40px;
        }

        &__text {
            margin-left: 10px;

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

        &:first-of-type {
            border-radius: 8px 8px 0 0;
        }

        &:last-of-type {
            border-radius: 0 0 8px 8px;
        }

        &:hover {
            background-color: #fafafb;

            h2,
            p {
                color: #0149ff;
            }
        }
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
