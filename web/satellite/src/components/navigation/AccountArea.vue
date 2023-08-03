// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="accountArea" class="account-area">
        <div role="button" tabindex="0" class="account-area__wrap" :class="{ active: isDropdown }" aria-roledescription="account-area" @keyup.enter="toggleDropdown" @click.stop="toggleDropdown">
            <div class="account-area__wrap__left">
                <AccountIcon class="account-area__wrap__left__icon" />
                <p class="account-area__wrap__left__label">My Account</p>
                <p class="account-area__wrap__left__label-small">Account</p>
                <TierBadgePro v-if="user.paidTier" class="account-area__wrap__left__tier-badge" />
                <TierBadgeFree v-else class="account-area__wrap__left__tier-badge" />
            </div>
            <ArrowImage class="account-area__wrap__arrow" />
        </div>
        <div v-if="isDropdown" v-click-outside="closeDropdown" class="account-area__dropdown" :style="style">
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
            <div v-if="!user.paidTier" tabindex="0" class="account-area__dropdown__item" @click="onUpgrade" @keyup.enter="onUpgrade">
                <UpgradeIcon />
                <p class="account-area__dropdown__item__label">Upgrade</p>
            </div>
            <div tabindex="0" class="account-area__dropdown__item" @click="navigateToBilling" @keyup.enter="navigateToBilling">
                <BillingIcon />
                <p class="account-area__dropdown__item__label">Billing</p>
            </div>
            <div tabindex="0" class="account-area__dropdown__item" @click="navigateToSettings" @keyup.enter="navigateToSettings">
                <SettingsIcon />
                <p class="account-area__dropdown__item__label">Account Settings</p>
            </div>
            <div tabindex="0" class="account-area__dropdown__item" @click="onLogout" @keyup.enter="onLogout">
                <LogoutIcon />
                <p class="account-area__dropdown__item__label">Logout</p>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { User } from '@/types/users';
import { RouteConfig } from '@/types/router';
import { AuthHttpApi } from '@/api/auth';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_DROPDOWNS, MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
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
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import BillingIcon from '@/../static/images/navigation/billing.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowImage from '@/../static/images/navigation/arrowExpandRight.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import UpgradeIcon from '@/../static/images/navigation/upgrade.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const configStore = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const pmStore = useProjectMembersStore();
const notificationsStore = useNotificationsStore();
const analyticsStore = useAnalyticsStore();

const auth: AuthHttpApi = new AuthHttpApi();

const dropdownYPos = ref<number>(0);
const dropdownXPos = ref<number>(0);
const accountArea = ref<HTMLDivElement>();

/**
 * Returns bottom and left position of dropdown.
 */
const style = computed((): Record<string, string> => {
    return { top: `${dropdownYPos.value}px`, left: `${dropdownXPos.value}px` };
});

/**
 * Indicates if account dropdown is visible.
 */
const isDropdown = computed((): boolean => {
    return appStore.state.activeDropdown === APP_STATE_DROPDOWNS.ACCOUNT;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return configStore.state.config.satelliteName;
});

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Starts upgrade account flow.
 */
function onUpgrade(): void {
    closeDropdown();

    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Navigates user to billing page.
 */
function navigateToBilling(): void {
    closeDropdown();

    if (route.path.includes(RouteConfig.Billing.path)) return;

    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
    analyticsStore.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
}

/**
 * Navigates user to account settings page.
 */
function navigateToSettings(): void {
    closeDropdown();
    analyticsStore.pageVisit(RouteConfig.Account.with(RouteConfig.Settings).path);
    router.push(RouteConfig.Account.with(RouteConfig.Settings).path).catch(() => {return;});
}

/**
 * Logouts user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    analyticsStore.pageVisit(RouteConfig.Login.path);
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
        analyticsStore.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
        await auth.logout();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.NAVIGATION_ACCOUNT_AREA);
    }
}

/**
 * Toggles account dropdown visibility.
 */
function toggleDropdown(): void {
    if (!accountArea.value) {
        return;
    }

    const DROPDOWN_HEIGHT = 224; // pixels
    const SIXTEEN_PIXELS = 16;
    const TWENTY_PIXELS = 20;
    const SEVENTY_PIXELS = 70;
    const accountContainer = accountArea.value.getBoundingClientRect();

    dropdownYPos.value = accountContainer.bottom - DROPDOWN_HEIGHT - (usersStore.state.user.paidTier ? SIXTEEN_PIXELS : SEVENTY_PIXELS);
    dropdownXPos.value = accountContainer.right - TWENTY_PIXELS;

    appStore.toggleActiveDropdown(APP_STATE_DROPDOWNS.ACCOUNT);
}

/**
 * Closes dropdowns.
 */
function closeDropdown(): void {
    appStore.closeDropdowns();
}
</script>

<style scoped lang="scss">
    .account-area {
        width: 100%;
        margin-top: 40px;

        &__wrap {
            box-sizing: border-box;
            padding: 22px 32px;
            outline: none;
            border: none;
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

            &:hover {
                background-color: var(--c-grey-1);
                border-color: var(--c-grey-1);

                p {
                    color: var(--c-blue-3);
                }

                .account-area__wrap__arrow :deep(path),
                .account-area__wrap__left__icon :deep(path) {
                    fill: var(--c-blue-3);
                }
            }

            &:focus {
                outline: none;
                border-color: var(--c-grey-1);
                background-color: var(--c-grey-1);
                color: var(--c-blue-3);

                p {
                    color: var(--c-blue-3);
                }

                :deep(path) {
                    fill: var(--c-blue-3);
                }
            }
        }

        &__dropdown {
            position: absolute;
            background: #fff;
            min-width: 240px;
            max-width: 240px;
            z-index: 1;
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
                    background-color: #f5f6fa;

                    p {
                        color: var(--c-blue-3);
                    }

                    :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }

                &:focus {
                    background-color: #f5f6fa;
                }
            }
        }
    }

    .active {
        border-color: #000;

        p {
            color: var(--c-blue-6);
            font-family: 'font_bold', sans-serif;
        }

        .account-area__wrap__arrow :deep(path),
        .account-area__wrap__left__icon :deep(path) {
            fill: #000;
        }
    }

    .active:hover {
        border-color: var(--c-blue-3);
        background-color: #f7f8fb;

        p {
            color: var(--c-blue-3);
        }

        .account-area__wrap__arrow :deep(path),
        .account-area__wrap__left__icon :deep(path) {
            fill: var(--c-blue-3);
        }
    }

    @media screen and (width <= 1280px) and (width >= 500px) {

        .account-area__wrap {
            padding: 10px 0;
            align-items: center;
            justify-content: center;

            p {
                font-family: 'font_medium', sans-serif;
            }

            &__left__label,
            &__arrow {
                display: none;
            }

            &__left {
                flex-direction: column;

                &__label-small {
                    display: block;
                    margin-top: 10px;
                    font-size: 9px;
                }
            }
        }

        .active p {
            font-family: 'font_medium', sans-serif;
        }
    }
</style>
