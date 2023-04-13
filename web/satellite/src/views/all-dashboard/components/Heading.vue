// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content">
            <LogoIcon class="header__content__logo" @click="goToProjects" />
            <div class="header__content__actions">
                <VButton
                    class="header__content__actions__docs"
                    icon="resources"
                    is-white
                    :link="link"
                    :on-press="sendDocsEvent"
                    label="Go to Docs"
                />
                <my-account-button />
            </div>
        </div>
        <div class="header__mobile-area">
            <div class="header__mobile-area__container">
                <header class="header__mobile-area__container__header">
                    <div class="header__mobile-area__container__header__logo" @click.stop="goToProjects">
                        <LogoIcon />
                    </div>
                    <CrossIcon v-if="isNavOpened" @click="toggleNavigation" />
                    <MenuIcon v-else @click="toggleNavigation" />
                </header>
                <div v-if="isNavOpened" class="header__mobile-area__container__wrap">
                    <a
                        aria-label="Docs"
                        class="header__mobile-area__container__wrap__item-container"
                        :href="link"
                        target="_blank"
                        rel="noopener noreferrer"
                        @click="sendDocsEvent"
                    >
                        <div class="header__mobile-area__container__wrap__item-container__left">
                            <resources-icon class="header__mobile-area__container__wrap__item-container__left__image" />
                            <p class="header__mobile-area__container__wrap__item-container__left__label">Go to Docs</p>
                        </div>
                    </a>
                    <div class="header__mobile-area__container__wrap__border" />
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
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { useNotify, useRouter } from '@/utils/hooks';
import MyAccountButton from '@/views/all-dashboard/components/MyAccountButton.vue';
import {
    AnalyticsErrorEventSource,
    AnalyticsEvent,
} from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { User } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotificationsStore } from '@/store/modules/notificationsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import ResourcesIcon from '@/../static/images/navigation/resources.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowIcon from '@/../static/images/navigation/arrowExpandRight.svg';
import CrossIcon from '@/../static/images/common/closeCross.svg';
import MenuIcon from '@/../static/images/navigation/menu.svg';

const router = useRouter();
const notify = useNotify();

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const billingStore = useBillingStore();
const projectsStore = useProjectsStore();
const notificationsStore = useNotificationsStore();
const obStore = useObjectBrowserStore();

const analytics = new AnalyticsHttpApi();
const auth = new AuthHttpApi();

const link = 'https://docs.storj.io/';

const isAccountDropdownShown = ref(false);
const isNavOpened = ref(false);

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return appStore.state.config.satelliteName;
});

/**
 * Returns user from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Toggles account dropdown visibility.
 */
function toggleAccountDropdown(): void {
    isAccountDropdownShown.value = !isAccountDropdownShown.value;
}

/**
 * Toggles navigation content visibility.
 */
function toggleNavigation(): void {
    isNavOpened.value = !isNavOpened.value;
}

/**
 * Sends "View Docs" event to segment and opens link.
 */
function goToProjects(): void {
    toggleNavigation();
    // this will close MyAccountButton.vue if it's open.
    appStore.closeDropdowns();

    const projects = RouteConfig.AllProjectsDashboard.path;
    if (router.currentRoute.path.includes(projects)) {
        return;
    }

    analytics.pageVisit(projects);
    router.push(projects);
}

function navigateToBilling(): void {
    toggleNavigation();

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
    toggleNavigation();

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
        projectsStore.clear(),
        usersStore.clear(),
        agStore.stopWorker(),
        agStore.clear(),
        notificationsStore.clear(),
        bucketsStore.clear(),
        appStore.clear(),
        billingStore.clear(),
        abTestingStore.reset(),
        obStore.clear(),
    ]);

    try {
        analytics.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.NAVIGATION_ACCOUNT_AREA);
    }
}

/**
 * Sends "View Docs" event to segment and opens link.
 */
function sendDocsEvent(): void {
    analytics.pageVisit(link);
    analytics.eventTriggered(AnalyticsEvent.VIEW_DOCS_CLICKED);
}
</script>

<style scoped lang="scss">
.header {

    &__content {
        display: flex;
        justify-content: space-between;
        align-items: center;

        &__logo {
            cursor: pointer;
            min-height: 37px;
            width: 207px;
            height: 37px;
        }

        &__actions {
            display: flex;
            gap: 10px;

            &__docs {
                padding: 10px 16px;
                border-radius: 8px;
            }
        }
    }

    &__mobile-area {
        background-color: #fff;
        font-family: 'font_regular', sans-serif;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
        display: none;

        &__container {
            position: relative;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: space-between;
            overflow-x: hidden;
            overflow-y: auto;
            width: 100%;

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

    @media screen and (max-width: 500px) {

        &__content {
            display: none;
        }

        &__mobile-area {
            display: block;
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
