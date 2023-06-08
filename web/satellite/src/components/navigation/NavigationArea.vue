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

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/types/router';
import { NavigationLink } from '@/types/navigation';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';

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

const configStore = useConfigStore();
const appStore = useAppStore();
const router = useRouter();
const route = useRoute();

const TWENTY_PIXELS = 20;
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
const navigation: NavigationLink[] = [
    RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
    RouteConfig.Buckets.withIcon(BucketsIcon),
    RouteConfig.AccessGrants.withIcon(AccessGrantsIcon),
    RouteConfig.Team.withIcon(UsersIcon),
];

const resourcesDropdownYPos = ref<number>(0);
const resourcesDropdownXPos = ref<number>(0);
const quickStartDropdownYPos = ref<number>(0);
const quickStartDropdownXPos = ref<number>(0);
const resourcesContainer = ref<HTMLDivElement>();
const quickStartContainer = ref<HTMLDivElement>();
const navigationContainer = ref<HTMLDivElement>();
const windowWidth = ref<number>(window.innerWidth);

/**
 * Indicates if resources dropdown shown.
 */
const isResourcesDropdownShown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.RESOURCES;
});

/**
 * Indicates if quick start dropdown shown.
 */
const isQuickStartDropdownShown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.QUICK_START;
});

/**
 * Indicates if all projects dashboard should be used.
 */
const isAllProjectsDashboard = computed((): boolean => {
    return configStore.state.config.allProjectsDashboard;
});

/**
 * On screen resize handler.
 */
function onResize(): void {
    windowWidth.value = window.innerWidth;
    closeDropdowns();
}

/**
 * Redirects to project dashboard.
 */
function onLogoClick(): void {
    if (isAllProjectsDashboard.value) {
        router.push(RouteConfig.AllProjectsDashboard.path);
        return;
    }

    if (route.name === RouteConfig.ProjectDashboard.name) {
        return;
    }

    router.push(RouteConfig.ProjectDashboard.path);
}

/**
 * Sets resources dropdown Y position depending on container's current position.
 * It is used to handle small screens.
 */
function setResourcesDropdownYPos(): void {
    if (!resourcesContainer.value) {
        return;
    }

    const container = resourcesContainer.value.getBoundingClientRect();
    resourcesDropdownYPos.value =  container.top + container.height / 2;
}

/**
 * Sets resources dropdown X position depending on container's current position.
 * It is used to handle small screens.
 */
function setResourcesDropdownXPos(): void {
    if (!resourcesContainer.value) {
        return;
    }

    resourcesDropdownXPos.value = resourcesContainer.value.getBoundingClientRect().width - TWENTY_PIXELS;
}

/**
 * Sets quick start dropdown Y position depending on container's current position.
 * It is used to handle small screens.
 */
function setQuickStartDropdownYPos(): void {
    if (!quickStartContainer.value) {
        return;
    }

    const container = quickStartContainer.value.getBoundingClientRect();
    quickStartDropdownYPos.value =  container.top + container.height / 2;
}

/**
 * Sets quick start dropdown X position depending on container's current position.
 * It is used to handle small screens.
 */
function setQuickStartDropdownXPos(): void {
    if (!quickStartContainer.value) {
        return;
    }

    quickStartDropdownXPos.value = quickStartContainer.value.getBoundingClientRect().width - TWENTY_PIXELS;
}

/**
 * Toggles resources dropdown visibility.
 */
function toggleResourcesDropdown(): void {
    setResourcesDropdownYPos();
    setResourcesDropdownXPos();
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.RESOURCES);
}

/**
 * Toggles quick start dropdown visibility.
 */
function toggleQuickStartDropdown(): void {
    setQuickStartDropdownYPos();
    setQuickStartDropdownXPos();
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.QUICK_START);
}

/**
 * Closes dropdowns.
 */
function closeDropdowns(): void {
    appStore.closeDropdowns();
}

/**
 * Sends new path click event to segment.
 */
function trackClickEvent(path: string): void {
    if (path === '/account/billing') {
        routeToOverview();
    } else {
        analytics.pageVisit(path);
    }
}

/**
 * Routes for new billing screens.
 */
function routeToOverview(): void {
    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
}

/**
 * Mounted hook after initial render.
 * Adds scroll event listener to close dropdowns.
 */
onMounted(() => {
    navigationContainer.value?.addEventListener('scroll', closeDropdowns);
    window.addEventListener('resize', onResize);
});

/**
 * Mounted hook before component destroy.
 * Removes scroll event listener.
 */
onBeforeUnmount(() => {
    navigationContainer.value?.removeEventListener('scroll', closeDropdowns);
    window.removeEventListener('resize', onResize);
});
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

    @media screen and (width <= 1280px) {

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
