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

import { User } from '@/types/users';
import { RouteConfig } from '@/router';
import { AuthHttpApi } from '@/api/auth';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { APP_STATE_DROPDOWNS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { useABTestingStore } from '@/store/modules/abTestingStore';

import BillingIcon from '@/../static/images/navigation/billing.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowImage from '@/../static/images/navigation/arrowExpandRight.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';

const store = useStore();
const router = useRouter();
const notify = useNotify();
const abTestingStore = useABTestingStore();
const auth: AuthHttpApi = new AuthHttpApi();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

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
    return store.state.appStateModule.viewsState.activeDropdown === APP_STATE_DROPDOWNS.ACCOUNT;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return store.state.appStateModule.satelliteName;
});

/**
 * Returns user entity from store.
 */
const user = computed((): User => {
    return store.getters.user;
});

/**
 * Navigates user to billing page.
 */
function navigateToBilling(): void {
    closeDropdown();

    if (router.currentRoute.path.includes(RouteConfig.Billing.path)) return;

    router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
    analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);
}

/**
 * Navigates user to account settings page.
 */
function navigateToSettings(): void {
    closeDropdown();
    analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Settings).path);
    router.push(RouteConfig.Account.with(RouteConfig.Settings).path).catch(() => {return;});
}

/**
 * Logouts user and navigates to login page.
 */
async function onLogout(): Promise<void> {
    analytics.pageVisit(RouteConfig.Login.path);
    router.push(RouteConfig.Login.path);

    await Promise.all([
        store.dispatch(PM_ACTIONS.CLEAR),
        store.dispatch(PROJECTS_ACTIONS.CLEAR),
        store.dispatch(USER_ACTIONS.CLEAR),
        store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER),
        store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR),
        store.dispatch(NOTIFICATION_ACTIONS.CLEAR),
        store.dispatch(BUCKET_ACTIONS.CLEAR),
        store.dispatch(OBJECTS_ACTIONS.CLEAR),
        store.dispatch(APP_STATE_ACTIONS.CLEAR),
        store.dispatch(PAYMENTS_ACTIONS.CLEAR_PAYMENT_INFO),
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
    const accountContainer = accountArea.value.getBoundingClientRect();

    dropdownYPos.value = accountContainer.bottom - DROPDOWN_HEIGHT - SIXTEEN_PIXELS;
    dropdownXPos.value = accountContainer.right - TWENTY_PIXELS;

    store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN, APP_STATE_DROPDOWNS.ACCOUNT);
    store.commit(APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION);
}

/**
 * Closes dropdowns.
 */
function closeDropdown(): void {
    store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
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

    @media screen and (max-width: 1280px) and (min-width: 500px) {

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
