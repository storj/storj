// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="header">
        <div class="header__content">
            <LogoIcon class="header__content__logo" @click="goToProjects" />
            <SmallLogoIcon class="header__content__small-logo" @click="goToProjects" />
            <div class="header__content__actions">
                <VButton
                    v-if="isMyProjectsButtonShown"
                    class="header__content__actions__button"
                    icon="project"
                    border-radius="8px"
                    font-size="12px"
                    is-white
                    :on-press="goToProjects"
                    label="My Projects"
                />
                <VButton
                    class="header__content__actions__button"
                    icon="resources"
                    border-radius="8px"
                    font-size="12px"
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
                    <LogoIcon class="header__mobile-area__container__header__logo" @click.stop="goToProjects" />
                    <CrossIcon v-if="isNavOpened" @click="toggleNavigation" />
                    <MenuIcon v-else @click="toggleNavigation" />
                </header>
                <div v-if="isNavOpened" class="header__mobile-area__container__wrap">
                    <div
                        v-if="isMyProjectsButtonShown"
                        aria-label="My Projects"
                        class="header__mobile-area__container__wrap__item-container"
                        @click="goToProjects"
                    >
                        <div class="header__mobile-area__container__wrap__item-container__left">
                            <project-icon class="header__mobile-area__container__wrap__item-container__left__image" />
                            <p class="header__mobile-area__container__wrap__item-container__left__label">My Projects</p>
                        </div>
                    </div>
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
import { useRoute, useRouter } from 'vue-router';

import { useNotify } from '@/utils/hooks';
import MyAccountButton from '@/views/all-dashboard/components/MyAccountButton.vue';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/types/router';
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
import { useConfigStore } from '@/store/modules/configStore';

import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import SmallLogoIcon from '@/../static/images/smallLogo.svg';
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
import ProjectIcon from '@/../static/images/navigation/project.svg';

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const configStore = useConfigStore();
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
    return configStore.state.config.satelliteName;
});

/**
 * Returns user from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns whether the My Projects button should be shown.
 */
const isMyProjectsButtonShown = computed((): boolean => {
    return route.name !== RouteConfig.AllProjectsDashboard.name;
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
    if (route.path.includes(projects)) {
        return;
    }

    analytics.pageVisit(projects);
    router.push(projects);
}

function navigateToBilling(): void {
    toggleNavigation();

    const billing = RouteConfig.AccountSettings.with(RouteConfig.Billing2);
    if (route.path.includes(billing.path)) {
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
    if (route.path.includes(settings)) {
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
        notify.notifyError(error, AnalyticsErrorEventSource.NAVIGATION_ACCOUNT_AREA);
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

            @media screen and (width <= 680px) {
                display: none;
            }
        }

        &__small-logo {
            cursor: pointer;
            width: 44px;
            height: 44px;
            display: none;

            @media screen and (width <= 680px) {
                display: block;
            }
        }

        &__actions {
            display: flex;
            gap: 10px;

            &__button {
                padding: 10px 16px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);

                :deep(.label) {

                    & > svg {
                        height: 14px;
                        width: 14px;
                    }

                    color: var(--c-black) !important;
                    font-weight: 700;
                    line-height: 20px;
                }
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
            width: 100%;

            &__header {
                display: flex;
                width: 100%;
                box-sizing: border-box;
                padding: 0 20px;
                justify-content: space-between;
                align-items: center;
                height: 4rem;

                &__logo {
                    height: 30px;
                    width: auto;
                }
            }

            &__wrap {
                position: absolute;
                top: 4rem;
                left: 0;
                display: flex;
                flex-direction: column;
                align-items: stretch;
                width: 100%;
                padding-bottom: 8px;
                z-index: 9999;
                overflow-y: auto;
                overflow-x: hidden;
                background: white;
                border-bottom: 1px solid var(--c-grey-2);

                &__item-container {
                    padding: 14px 32px;
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
                    margin: 8px 32px;
                    height: 1px;
                    background-color: var(--c-grey-2);
                }
            }
        }
    }

    @media screen and (width <= 500px) {

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
