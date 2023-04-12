// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        class="account-button"
        @click.stop.prevent="openDropdown"
        @mouseenter="isHoveredOver = true"
        @mouseleave="isHoveredOver = false"
    >
        <div class="account-button__button">
            <account-icon class="account-button__button__icon" :class="{active: isHoveredOver || isDropdownOpen}" />
            <span class="account-button__button__label" :class="{active: isHoveredOver || isDropdownOpen}">My Account</span>
            <arrow-down-icon class="account-button__arrow" :class="{active: isDropdownOpen, hovered: isHoveredOver}" />
        </div>
        <div v-if="isDropdownOpen" v-click-outside="closeDropdown" class="account-button__dropdown">
            <div class="account-button__dropdown__header">
                <div class="account-button__dropdown__header__left">
                    <SatelliteIcon />
                    <h2 class="account-button__dropdown__header__left__label">Account Region</h2>
                </div>
                <div class="account-button__dropdown__header__right">
                    <p class="account-button__dropdown__header__right__sat">{{ satellite }}</p>
                    <a
                        href="https://docs.storj.io/dcs/concepts/satellite"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="account-button__dropdown__header__right__link"
                        @click.stop="closeDropdown"
                    >
                        <InfoIcon />
                    </a>
                </div>
            </div>
            <div tabindex="0" class="account-button__dropdown__item" @click.stop="navigateToBilling" @keyup.enter="navigateToBilling">
                <BillingIcon />
                <p class="account-button__dropdown__item__label">Billing</p>
            </div>
            <div tabindex="0" class="account-button__dropdown__item" @click.stop="navigateToSettings" @keyup.enter="navigateToSettings">
                <SettingsIcon />
                <p class="account-button__dropdown__item__label">Account Settings</p>
            </div>
            <div tabindex="0" class="account-button__dropdown__item" @click.stop="onLogout" @keyup.enter="onLogout">
                <LogoutIcon />
                <p class="account-button__dropdown__item__label">Logout</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { RouteConfig } from '@/router';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import {
    AnalyticsErrorEventSource,
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AuthHttpApi } from '@/api/auth';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';

import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowDownIcon from '@/../static/images/common/dropIcon.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';

const router = useRouter();
const store = useStore();
const notify = useNotify();

const analytics = new AnalyticsHttpApi();
const auth = new AuthHttpApi();

const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const pmStore = useProjectMembersStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();

const isHoveredOver = ref(false);

/**
 * Indicates if account dropdown is open.
 */
const isDropdownOpen = computed((): boolean => {
    return appStore.state.viewsState.activeDropdown === APP_STATE_DROPDOWNS.ALL_DASH_ACCOUNT;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return appStore.state.satelliteName;
});

function openDropdown(): void {
    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.ALL_DASH_ACCOUNT);
}

function closeDropdown(): void {
    appStore.closeDropdowns();
}

/**
 * Navigates user to billing page.
 */
function navigateToBilling(): void {
    closeDropdown();

    const billing = RouteConfig.AccountSettings.with(RouteConfig.Billing2);
    if (router.currentRoute.path.includes(billing.path)) {
        return;
    }

    const routeConf = billing.with(RouteConfig.BillingOverview2).path;
    router.push(routeConf);
    analytics.pageVisit(routeConf);
}

/**
 * Navigates user to account settings page.
 */
function navigateToSettings(): void {
    closeDropdown();
    const settings = RouteConfig.AccountSettings.with(RouteConfig.Settings2).path;
    if (router.currentRoute.path.includes(settings)) {
        return;
    }

    analytics.pageVisit(settings);
    router.push(settings).catch(() => {return;});
}

/**
 * Logouts user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    analytics.pageVisit(RouteConfig.Login.path);
    await router.push(RouteConfig.Login.path);

    await Promise.all([
        pmStore.clear(),
        store.dispatch(PROJECTS_ACTIONS.CLEAR),
        usersStore.clear(),
        agStore.stopWorker(),
        agStore.clear(),
        store.dispatch(NOTIFICATION_ACTIONS.CLEAR),
        store.dispatch(BUCKET_ACTIONS.CLEAR),
        store.dispatch(OBJECTS_ACTIONS.CLEAR),
        appStore.clear(),
        billingStore.clear(),
        abTestingStore.reset(),
        store.dispatch('files/clear'),
    ]);

    try {
        analytics.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_ACCOUNT_AREA);
    }
}
</script>

<style scoped lang="scss">
.account-button {
    position: relative;
    display: flex;
    align-items: center;
    padding: 14px 18px;
    box-sizing: border-box;
    color: var(--c-grey-6);
    cursor: pointer;
    background: var(--c-white);
    border: 1px solid var(--c-grey-3);
    border-radius: 8px;
    height: 44px;

    &:hover,
    &:active,
    &:focus {
        border: 1px solid var(--c-blue-3);
    }

    &__button {
        display: flex;
        align-items: center;
        justify-content: space-evenly;
        cursor: pointer;

        &__icon {
            transition-duration: 0.5s;
            margin-right: 10px;
        }

        &__label {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 23px;
            color: var(--c-grey-6);
            margin-right: 10px;
            white-space: nowrap;
        }

        &__label.active {
            color: var(--c-blue-3);
        }

        &__icon.active {

            :deep(path) {
                fill: var(--c-blue-3);
            }
        }
    }

    &__dropdown {
        position: absolute;
        top: 50px;
        right: 0;
        background: var(--c-white);
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        font-weight: normal;
        width: 270px;
        z-index: 999;
        cursor: default;
        border: 1px solid var(--c-grey-2);
        box-sizing: border-box;
        box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
        border-radius: 8px;

        &__header {
            background: var(--c-grey-1);
            padding: 16px;
            width: calc(100% - 32px);
            border: 1px solid var(--c-grey-2);
            display: flex;
            align-items: center;
            justify-content: space-between;
            border-radius: 8px 8px 0 0;

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

                &__link:focus {

                    svg :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }
            }
        }

        &__item {
            display: flex;
            align-items: center;
            border-top: 1px solid var(--c-grey-2);
            padding: 16px;
            width: calc(100% - 32px);
            cursor: pointer;

            &__label {
                margin-left: 16px;
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
            }

            &:last-of-type {
                border-radius: 0 0 8px 8px;
            }

            &:hover {
                background-color: var(--c-grey-1);

                p {
                    color: var(--c-blue-3);
                }

                :deep(path) {
                    fill: var(--c-blue-3);
                }
            }

            &:focus {
                background-color: var(--c-grey-1);
            }
        }
    }

    &__arrow {
        transition-duration: 0.5s;
    }

    &__arrow.active {
        transform: rotate(180deg) scaleX(-1);

        :deep(path) {
            fill: var(--c-blue-3);
        }
    }

    &__arrow.hovered {

        :deep(path) {
            fill: var(--c-blue-3);
        }
    }
}
</style>
