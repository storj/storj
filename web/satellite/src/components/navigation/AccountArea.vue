// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="accountArea" class="account-area">
        <div class="account-area__wrap" :class="{ active: isDropdown }" aria-roledescription="account-area" @click.stop="toggleDropdown">
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
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { User } from '@/types/users';
import { RouteConfig } from '@/router';
import { LocalData } from '@/utils/localData';
import { AuthHttpApi } from '@/api/auth';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { MetaUtils } from '@/utils/meta';

import BillingIcon from '@/../static/images/navigation/billing.svg';
import InfoIcon from '@/../static/images/navigation/info.svg';
import SatelliteIcon from '@/../static/images/navigation/satellite.svg';
import AccountIcon from '@/../static/images/navigation/account.svg';
import ArrowImage from '@/../static/images/navigation/arrowExpandRight.svg';
import SettingsIcon from '@/../static/images/navigation/settings.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';
import TierBadgeFree from '@/../static/images/navigation/tierBadgeFree.svg';
import TierBadgePro from '@/../static/images/navigation/tierBadgePro.svg';

// @vue/component
@Component({
    components: {
        InfoIcon,
        SatelliteIcon,
        AccountIcon,
        ArrowImage,
        BillingIcon,
        SettingsIcon,
        LogoutIcon,
        TierBadgeFree,
        TierBadgePro,
    },
})
export default class AccountArea extends Vue {
    private readonly auth: AuthHttpApi = new AuthHttpApi();
    private dropdownYPos = 0;
    private dropdownXPos = 0;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public $refs!: {
        accountArea: HTMLDivElement,
    };

    /**
     * Navigates user to billing page.
     */
    public navigateToBilling(): void {
        this.closeDropdown();
        if (this.$route.path.includes(RouteConfig.Billing.path)) return;

        this.isNewBillingScreen ?
            this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path) :
            this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).path);

        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).path);
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Billing).with(RouteConfig.BillingOverview).path);

    }

    /**
     * Navigates user to account settings page.
     */
    public navigateToSettings(): void {
        this.closeDropdown();
        this.analytics.pageVisit(RouteConfig.Account.with(RouteConfig.Settings).path);
        this.$router.push(RouteConfig.Account.with(RouteConfig.Settings).path).catch(() => {return;});
    }

    /**
     * Logouts user and navigates to login page.
     */
    public async onLogout(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.Login.path);
        await this.$router.push(RouteConfig.Login.path);

        try {
            this.analytics.eventTriggered(AnalyticsEvent.LOGOUT_CLICKED);
            await this.auth.logout();
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        await this.$store.dispatch(PM_ACTIONS.CLEAR);
        await this.$store.dispatch(PROJECTS_ACTIONS.CLEAR);
        await this.$store.dispatch(USER_ACTIONS.CLEAR);
        await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR);
        await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER);
        await this.$store.dispatch(NOTIFICATION_ACTIONS.CLEAR);
        await this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
        await this.$store.dispatch(OBJECTS_ACTIONS.CLEAR);
        await this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);

        LocalData.removeUserId();
    }

    /**
     * Toggles account dropdown visibility.
     */
    public toggleDropdown(): void {
        const DROPDOWN_HEIGHT = 224; // pixels
        const SIXTEEN_PIXELS = 16;
        const TWENTY_PIXELS = 20;
        const accountContainer = this.$refs.accountArea.getBoundingClientRect();

        this.dropdownYPos = accountContainer.bottom - DROPDOWN_HEIGHT - SIXTEEN_PIXELS;
        this.dropdownXPos = accountContainer.right - TWENTY_PIXELS;

        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
        this.$store.commit(APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION);
    }

    /**
     * Closes dropdowns.
     */
    public closeDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Indicates if tabs options are hidden.
     */
    public get isNewBillingScreen(): boolean {
        const isNewBillingScreen = MetaUtils.getMetaContent('new-billing-screen');
        return isNewBillingScreen === 'true';
    }

    /**
     * Returns bottom and left position of dropdown.
     */
    public get style(): Record<string, string> {
        return { top: `${this.dropdownYPos}px`, left: `${this.dropdownXPos}px` };
    }

    /**
     * Indicates if account dropdown is visible.
     */
    public get isDropdown(): boolean {
        return this.$store.state.appStateModule.appState.isAccountDropdownShown;
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
    .account-area {
        width: 100%;
        margin-top: 40px;

        &__wrap {
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
                justify-content: space-between;

                &__label,
                &__label-small {
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
                    margin: 0 6px 0 24px;
                }

                &__label-small {
                    display: none;
                    margin: 0;
                }
            }

            &:hover {
                background-color: #fafafb;
                border-color: #fafafb;

                p {
                    color: #0149ff;
                }

                .account-area__wrap__arrow :deep(path),
                .account-area__wrap__left__icon :deep(path) {
                    fill: #0149ff;
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
            border: 1px solid #ebeef1;
            box-sizing: border-box;
            box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
            border-radius: 8px;

            &__header {
                background: #fafafb;
                padding: 16px;
                width: calc(100% - 32px);
                border: 1px solid #ebeef1;
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
                        color: #56606d;
                        margin-left: 16px;
                    }

                    &__sat {
                        font-size: 14px;
                        line-height: 20px;
                        color: #56606d;
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
                border-top: 1px solid #ebeef1;
                padding: 16px;
                width: calc(100% - 32px);
                cursor: pointer;

                &__label {
                    margin-left: 16px;
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
                }

                &:last-of-type {
                    border-radius: 0 0 8px 8px;
                }

                &:hover {
                    background-color: #f7f8fb;

                    p {
                        color: #0149ff;
                    }

                    :deep(path) {
                        fill: #0149ff;
                    }
                }
            }
        }
    }

    .active {
        border-color: #000;

        p {
            color: #091c45;
            font-family: 'font_bold', sans-serif;
        }

        .account-area__wrap__arrow :deep(path),
        .account-area__wrap__left__icon :deep(path) {
            fill: #000;
        }
    }

    .active:hover {
        border-color: #0149ff;
        background-color: #f7f8fb;

        p {
            color: #0149ff;
        }

        .account-area__wrap__arrow :deep(path),
        .account-area__wrap__left__icon :deep(path) {
            fill: #0149ff;
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
