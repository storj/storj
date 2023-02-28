// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="navigation-area">
        <div class="navigation-area__container">
            <header class="navigation-area__container__header">
                <div class="navigation-area__container__header__logo" @click.stop="onLogoClick">
                    <LogoIcon />
                </div>
                <CrossIcon v-if="isOpened" @click="toggleNavigation" />
                <MenuIcon v-else @click="toggleNavigation" />
            </header>
            <div v-if="isOpened" class="navigation-area__container__wrap" :class="{ 'with-padding': isAccountDropdownShown }">
                <div class="navigation-area__container__wrap__edit">
                    <div
                        class="project-selection__selected"
                        aria-roledescription="project-selection"
                        @click.stop.prevent="onProjectClick"
                    >
                        <div class="project-selection__selected__left">
                            <ProjectIcon class="project-selection__selected__left__image" />
                            <p class="project-selection__selected__left__name" :title="selectedProject.name">{{ selectedProject.name }}</p>
                            <p class="project-selection__selected__left__placeholder">Projects</p>
                        </div>
                        <ArrowIcon class="project-selection__selected__arrow" />
                    </div>
                    <div v-if="isProjectDropdownShown" class="project-selection__dropdown">
                        <div v-if="isLoading" class="project-selection__dropdown__loader-container">
                            <VLoader width="30px" height="30px" />
                        </div>
                        <div v-else class="project-selection__dropdown__items">
                            <div class="project-selection__dropdown__items__choice" @click.prevent.stop="toggleProjectDropdown">
                                <div class="project-selection__dropdown__items__choice__mark-container">
                                    <CheckmarkIcon class="project-selection__dropdown__items__choice__mark-container__image" />
                                </div>
                                <p class="project-selection__dropdown__items__choice__selected">
                                    {{ selectedProject.name }}
                                </p>
                            </div>
                            <div
                                v-for="project in projects"
                                :key="project.id"
                                class="project-selection__dropdown__items__choice"
                                @click.prevent.stop="onProjectSelected(project.id)"
                            >
                                <p class="project-selection__dropdown__items__choice__unselected">{{ project.name }}</p>
                            </div>
                        </div>
                        <div tabindex="0" class="project-selection__dropdown__link-container" @click.stop="onManagePassphraseClick" @keyup.enter="onManagePassphraseClick">
                            <PassphraseIcon />
                            <p class="project-selection__dropdown__link-container__label">Manage Passphrase</p>
                        </div>
                        <div class="project-selection__dropdown__link-container" @click.stop="onProjectsLinkClick">
                            <ManageIcon />
                            <p class="project-selection__dropdown__link-container__label">Manage Projects</p>
                        </div>
                        <div class="project-selection__dropdown__link-container" @click.stop="onCreateLinkClick">
                            <CreateProjectIcon />
                            <p class="project-selection__dropdown__link-container__label">Create new</p>
                        </div>
                    </div>
                </div>
                <div v-if="!isProjectDropdownShown" class="navigation-area__container__wrap__border" />
                <router-link
                    v-for="navItem in navigation"
                    :key="navItem.name"
                    :aria-label="navItem.name"
                    class="navigation-area__container__wrap__item-container"
                    :to="navItem.path"
                    @click.native="onNavClick(navItem.path)"
                >
                    <div class="navigation-area__container__wrap__item-container__left">
                        <component :is="navItem.icon" class="navigation-area__container__wrap__item-container__left__image" />
                        <p class="navigation-area__container__wrap__item-container__left__label">{{ navItem.name }}</p>
                    </div>
                </router-link>
                <div class="navigation-area__container__wrap__border" />
                <div class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        @click.stop="toggleResourcesDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <ResourcesIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Resources</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <div
                        v-if="isResourcesDropdownShown"
                    >
                        <ResourcesLinks />
                    </div>
                </div>
                <div class="container-wrapper">
                    <div
                        class="navigation-area__container__wrap__item-container"
                        @click.stop="toggleQuickStartDropdown"
                    >
                        <div class="navigation-area__container__wrap__item-container__left">
                            <QuickStartIcon class="navigation-area__container__wrap__item-container__left__image" />
                            <p class="navigation-area__container__wrap__item-container__left__label">Quickstart</p>
                        </div>
                        <ArrowIcon class="navigation-area__container__wrap__item-container__arrow" />
                    </div>
                    <div
                        v-if="isQuickStartDropdownShown"
                    >
                        <QuickStartLinks />
                    </div>
                </div>
                <div class="account-area">
                    <div class="account-area__wrap" aria-roledescription="account-area" @click.stop="toggleAccountDropdown">
                        <div class="account-area__wrap__left">
                            <AccountIcon class="account-area__wrap__left__icon" />
                            <p class="account-area__wrap__left__label">My Account</p>
                            <p class="account-area__wrap__left__label-small">Account</p>
                            <TierBadgePro v-if="user.paidTier" class="account-area__wrap__left__tier-badge" />
                            <TierBadgeFree v-else class="account-area__wrap__left__tier-badge" />
                        </div>
                        <ArrowIcon class="account-area__wrap__arrow" />
                    </div>
                    <div v-if="isAccountDropdownShown" class="account-area__dropdown">
                        <div class="account-area__dropdown__header">
                            <div class="account-area__dropdown__header__left">
                                <SatelliteIcon />
                                <h2 class="account-area__dropdown__header__left__label">Satellite</h2>
                            </div>
                            <div class="account-area__dropdown__header__right">
                                <p class="account-area__dropdown__header__right__sat">{{ satellite }}</p>
                                <a
                                    class="account-area__dropdown__header__right__link"
                                    href="https://docs.storj.io/dcs/concepts/satellite"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    <InfoIcon />
                                </a>
                            </div>
                        </div>
                        <div class="account-area__dropdown__item" @click="navigateToBilling">
                            <BillingIcon />
                            <p class="account-area__dropdown__item__label">Billing</p>
                        </div>
                        <div class="account-area__dropdown__item" @click="navigateToSettings">
                            <SettingsIcon />
                            <p class="account-area__dropdown__item__label">Account Settings</p>
                        </div>
                        <div class="account-area__dropdown__item" @click="onLogout">
                            <LogoutIcon />
                            <p class="account-area__dropdown__item__label">Logout</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">

import { Component, Vue } from 'vue-property-decorator';

import { AuthHttpApi } from '@/api/auth';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { NavigationLink } from '@/types/navigation';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';
import { AB_TESTING_ACTIONS } from '@/store/modules/abTesting';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import ResourcesLinks from '@/components/navigation/ResourcesLinks.vue';
import QuickStartLinks from '@/components/navigation/QuickStartLinks.vue';
import ProjectSelection from '@/components/navigation/ProjectSelection.vue';
import AccountArea from '@/components/navigation/AccountArea.vue';
import VLoader from '@/components/common/VLoader.vue';

import CrossIcon from '@/../static/images/common/closeCross.svg';
import LogoIcon from '@/../static/images/logo.svg';
import AccessGrantsIcon from '@/../static/images/navigation/accessGrants.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowIcon from '@/../static/images/navigation/arrowExpandRight.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import BucketsIcon from '@/../static/images/navigation/buckets.svg';
import CheckmarkIcon from '@/../static/images/navigation/checkmark.svg';
import CreateProjectIcon from '@/../static/images/navigation/createProject.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import ManageIcon from '@/../static/images/navigation/manage.svg';
import PassphraseIcon from '@/../static/images/navigation/passphrase.svg';
import MenuIcon from '@/../static/images/navigation/menu.svg';
import ProjectIcon from '@/../static/images/navigation/project.svg';
import DashboardIcon from '@/../static/images/navigation/projectDashboard.svg';
import QuickStartIcon from '@/../static/images/navigation/quickStart.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';
import UsersIcon from '@/../static/images/navigation/users.svg';

// @vue/component
@Component({
    components: {
        ResourcesLinks,
        QuickStartLinks,
        ProjectSelection,
        AccountArea,
        LogoIcon,
        DashboardIcon,
        AccessGrantsIcon,
        UsersIcon,
        BillingIcon,
        BucketsIcon,
        ResourcesIcon,
        QuickStartIcon,
        ArrowIcon,
        CheckmarkIcon,
        ProjectIcon,
        ManageIcon,
        PassphraseIcon,
        CreateProjectIcon,
        VLoader,
        CrossIcon,
        MenuIcon,
        InfoIcon,
        SatelliteIcon,
        AccountIcon,
        SettingsIcon,
        LogoutIcon,
        TierBadgeFree,
        TierBadgePro,
    },
})
export default class MobileNavigation extends Vue {
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
    private readonly auth: AuthHttpApi = new AuthHttpApi();

    private FIRST_PAGE = 1;
    public isResourcesDropdownShown = false;
    public isQuickStartDropdownShown = false;
    public isProjectDropdownShown = false;
    public isAccountDropdownShown = false;
    public isOpened = false;
    public isLoading = false;

    public navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        RouteConfig.Buckets.withIcon(BucketsIcon),
        RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
        RouteConfig.Users.withIcon(UsersIcon),
    ];

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

    public onNavClick(path: string): void {
        this.trackClickEvent(path);
        this.isOpened = false;
    }

    /**
     * Toggles navigation content visibility.
     */
    public toggleNavigation(): void {
        this.isOpened = !this.isOpened;
    }

    /**
     * Toggles resources dropdown visibility.
     */
    public toggleResourcesDropdown(): void {
        this.isResourcesDropdownShown = !this.isResourcesDropdownShown;
    }

    /**
     * Toggles quick start dropdown visibility.
     */
    public toggleQuickStartDropdown(): void {
        this.isQuickStartDropdownShown = !this.isQuickStartDropdownShown;
    }

    /**
     * Toggles projects dropdown visibility.
     */
    public toggleProjectDropdown(): void {
        this.isProjectDropdownShown = !this.isProjectDropdownShown;
    }

    /**
     * Toggles account dropdown visibility.
     */
    public toggleAccountDropdown(): void {
        this.isAccountDropdownShown = !this.isAccountDropdownShown;
        window.scrollTo(0, document.querySelector('.navigation-area__container__wrap')?.scrollHeight || 0);
    }

    /**
     * Indicates if new project dashboard should be used.
     */
    public get isNewProjectDashboard(): boolean {
        return this.$store.state.appStateModule.isNewProjectDashboard;
    }

    /**
     * Returns projects list from store.
     */
    public get projects(): Project[] {
        return this.$store.getters.projectsWithoutSelected;
    }

    /**
     * Sends new path click event to segment.
     */
    public trackClickEvent(path: string): void {
        this.analytics.pageVisit(path);
    }

    /**
     * Toggles manage passphrase modal shown.
     */
    public onManagePassphraseClick(): void {
        this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.manageProjectPassphrase);
    }

    /**
     * Indicates if current route is objects view.
     */
    private get isBucketsView(): boolean {
        const currentRoute = this.$route.path;

        return currentRoute.includes(RouteConfig.BucketsManagement.path);
    }

    /**
     * Returns selected project from store.
     */
    public get selectedProject(): Project {
        return this.$store.getters.selectedProject;
    }

    public async onProjectClick(): Promise<void> {
        this.toggleProjectDropdown();

        if (this.isLoading || !this.isProjectDropdownShown) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
        } finally {
            this.isLoading = false;
        }
    }

    /**
     * Fetches all project related information.
     * @param projectID
     */
    public async onProjectSelected(projectID: string): Promise<void> {
        await this.analytics.eventTriggered(AnalyticsEvent.NAVIGATE_PROJECTS);
        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        LocalData.setSelectedProjectId(projectID);
        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
        this.isProjectDropdownShown = false;

        if (this.isBucketsView) {
            await this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
            this.analytics.pageVisit(RouteConfig.Buckets.path);
            await this.$router.push(RouteConfig.Buckets.path).catch(() => {return; });
        }

        try {
            await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.FIRST_PAGE);
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(`Unable to select project. ${error.message}`, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
        }
    }

    /**
     * Route to projects list page.
     */
    public onProjectsLinkClick(): void {
        if (this.$route.name !== RouteConfig.ProjectsList.name) {
            this.analytics.pageVisit(RouteConfig.ProjectsList.path);
            this.analytics.eventTriggered(AnalyticsEvent.MANAGE_PROJECTS_CLICKED);
            this.$router.push(RouteConfig.ProjectsList.path);
        }

        this.isProjectDropdownShown = false;
    }

    /**
     * Route to create project page.
     */
    public onCreateLinkClick(): void {
        if (this.$route.name !== RouteConfig.CreateProject.name) {
            this.analytics.eventTriggered(AnalyticsEvent.CREATE_NEW_CLICKED);

            const user: User = this.$store.getters.user;
            const ownProjectsCount: number = this.$store.getters.projectsCount;

            if (!user.paidTier && user.projectLimit === ownProjectsCount) {
                this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.createProjectPrompt);
            } else {
                this.analytics.pageVisit(RouteConfig.CreateProject.path);
                this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.createProject);
            }
        }

        this.isProjectDropdownShown = false;
    }

    /**
     * Navigates user to billing page.
     */
    public navigateToBilling(): void {
        this.isOpened = false;
        if (this.$route.path.includes(RouteConfig.Billing.path)) return;

        let link = RouteConfig.Account.with(RouteConfig.Billing);
        if (MetaUtils.getMetaContent('new-billing-screen') === 'true') {
            link = link.with(RouteConfig.BillingOverview);
        }
        this.$router.push(link.path);
        this.analytics.pageVisit(link.path);
    }

    /**
     * Navigates user to account settings page.
     */
    public navigateToSettings(): void {
        this.isOpened = false;
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Settings).path);
        this.$router.push(RouteConfig.Account.with(RouteConfig.Settings).path).catch(() => {return;});
    }

    /**
     * Logouts user and navigates to login page.
     */
    public async onLogout(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.Login.path);
        await this.$router.push(RouteConfig.Login.path);

        await Promise.all([
            this.$store.dispatch(PM_ACTIONS.CLEAR),
            this.$store.dispatch(PROJECTS_ACTIONS.CLEAR),
            this.$store.dispatch(USER_ACTIONS.CLEAR),
            this.$store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER),
            this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR),
            this.$store.dispatch(NOTIFICATION_ACTIONS.CLEAR),
            this.$store.dispatch(BUCKET_ACTIONS.CLEAR),
            this.$store.dispatch(OBJECTS_ACTIONS.CLEAR),
            this.$store.dispatch(APP_STATE_ACTIONS.CLEAR),
            this.$store.dispatch(PAYMENTS_ACTIONS.CLEAR_PAYMENT_INFO),
            this.$store.dispatch(AB_TESTING_ACTIONS.RESET),
            this.$store.dispatch('files/clear'),
        ]);

        try {
            this.analytics.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
            await this.auth.logout();
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.MOBILE_NAVIGATION);
        }
    }

    /**
     * Returns satellite name from store.
     */
    public get satellite(): boolean {
        return this.$store.state.appStateModule.satelliteName;
    }

    /**
     * Returns user entity from store.
     */
    public get user(): User {
        return this.$store.getters.user;
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
    background-color: #fff;
    font-family: 'font_regular', sans-serif;
    box-shadow: 0 0 32px rgb(0 0 0 / 4%);

    &__container {
        position: relative;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        overflow-x: hidden;
        overflow-y: auto;
        width: 100%;
        height: 100%;

        &__header {
            display: flex;
            width: 100%;
            box-sizing: border-box;
            padding: 0 32px;
            justify-content: space-between;
            align-items: center;
            height: 4rem;

            &__logo {
                width: 211px;
                max-width: 211px;
                height: 37px;
                max-height: 37px;

                svg {
                    width: 211px;
                    height: 37px;
                }
            }
        }

        &__wrap {
            position: fixed;
            top: 4rem;
            left: 0;
            display: flex;
            flex-direction: column;
            align-items: center;
            width: 100%;
            z-index: 9999;
            overflow-y: auto;
            overflow-x: hidden;
            background: white;
            height: calc(var(--vh, 100vh) - 4rem);

            &.with-padding {
                padding-bottom: 3rem;
                height: calc(var(--vh, 100vh) - 7rem);
            }

            &__small-logo {
                display: none;
            }

            &__edit {
                margin: 10px 0 16px;
                width: 100%;
            }

            &__item-container {
                padding: 14px 32px;
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
                border-left: 4px solid #fff;
                color: var(--c-grey-6);
                position: static;
                cursor: pointer;
                height: 48px;
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
                margin: 0 32px 16px;
                border: 0.5px solid var(--c-grey-2);
                width: calc(100% - 48px);
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
}

:deep(.dropdown-item) {
    display: flex;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    padding: 16px;
    cursor: pointer;
    border-bottom: 1px solid var(--c-grey-2);
    background: var(--c-grey-1);
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

:deep(.project-selection__dropdown) {
    all: unset !important;
    position: relative !important;
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;
    padding: 10px 16px;
    cursor: pointer;
    border-top: 1px solid var(--c-grey-2);
    border-bottom: 1px solid var(--c-grey-2);
}

.project-selection {
    font-family: 'font_regular', sans-serif;
    position: relative;
    width: 100%;

    &__selected {
        box-sizing: border-box;
        padding: 22px 32px;
        border-left: 4px solid #fff;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;
        position: static;

        &__left {
            display: flex;
            align-items: center;
            max-width: calc(100% - 16px);

            &__name {
                max-width: calc(100% - 24px - 16px);
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin-left: 24px;
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
            }

            &__placeholder {
                display: none;
            }
        }
    }

    &__dropdown {
        width: 100%;
        background-color: #fff;

        &__loader-container {
            margin: 10px 0;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 8px 8px 0 0;
        }

        &__items {
            overflow-y: auto;
            background-color: #fff;

            &__choice {
                display: flex;
                align-items: center;
                padding: 16px 32px;
                cursor: pointer;
                height: 32px;

                &__selected,
                &__unselected {
                    font-size: 14px;
                    line-height: 20px;
                    color: #1b2533;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }

                &__selected {
                    font-family: 'font_bold', sans-serif;
                    margin-left: 24px;
                }

                &__unselected {
                    padding-left: 40px;
                }

                &__mark-container {
                    width: 16px;
                    height: 16px;

                    &__image {
                        object-fit: cover;
                    }
                }
            }
        }

        &__link-container {
            padding: 16px 32px;
            height: 48px;
            cursor: pointer;
            display: flex;
            align-items: center;
            box-sizing: border-box;
            border-bottom: 1px solid var(--c-grey-2);

            &__label {
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin-left: 24px;
            }

            &:last-of-type {
                border-radius: 0 0 8px 8px;
            }
        }
    }
}

.account-area {
    width: 100%;

    &__wrap {
        box-sizing: border-box;
        padding: 16px 32px;
        height: 48px;
        width: 100%;
        display: flex;
        align-items: center;
        justify-content: space-between;
        cursor: pointer;
        position: static;

        &__left {
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__label,
            &__label-small {
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin: 0 6px 0 24px;
            }

            &__label-small {
                display: none;
                margin: 0;
            }
        }
    }

    &__dropdown {
        position: relative;
        background: #fff;
        width: 100%;
        box-sizing: border-box;

        &__header {
            background: var(--c-grey-1);
            padding: 16px 32px;
            border: 1px solid var(--c-grey-2);
            display: flex;
            align-items: center;
            justify-content: space-between;

            &__left,
            &__right {
                display: flex;
                align-items: center;

                &__label {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    margin-left: 16px;
                }

                &__sat {
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    margin-right: 16px;
                }

                &__link {
                    max-height: 16px;
                }
            }
        }

        &__item {
            display: flex;
            align-items: center;
            padding: 16px 32px;
            background: var(--c-grey-1);

            &__label {
                margin-left: 16px;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
            }

            &:last-of-type {
                border-radius: 0 0 8px 8px;
            }
        }
    }
}
</style>
