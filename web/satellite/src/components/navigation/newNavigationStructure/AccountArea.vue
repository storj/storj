// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div ref="accountArea" class="account-area" :class="{ active: isDropdown }" aria-roledescription="account-area" @click.stop="toggleDropdown">
        <div class="account-area__avatar">
            <h1 class="account-area__avatar__letter">{{ avatarLetter }}</h1>
        </div>
        <div class="account-area__info">
            <h2 class="account-area__info__email">{{ user.email }}</h2>
            <p v-if="user.paidTier" class="account-area__info__type">Pro account</p>
            <p v-else class="account-area__info__type">Free account</p>
        </div>
        <div v-if="isDropdown" v-click-outside="closeDropdown" class="account-area__dropdown" :style="style">
            <div class="account-area__dropdown__header">
                <p class="account-area__dropdown__header__greet">👋🏼 &nbsp; Hi, {{ user.fullName }}</p>
                <p class="account-area__dropdown__header__sat">{{ satellite }}</p>
            </div>
            <div class="account-area__dropdown__item" @click="navigateToSettings">
                <SettingsIcon />
                <p class="account-area__dropdown__item__label">Account Settings</p>
            </div>
            <div class="account-area__dropdown__item" @click="navigateToBilling">
                <BillingIcon />
                <p class="account-area__dropdown__item__label">Billing</p>
            </div>
            <div v-if="!user.paidTier" class="account-area__dropdown__item" @click="onUpgrade">
                <PlanIcon />
                <p class="account-area__dropdown__item__label">Upgrade Plan</p>
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

import { User } from "@/types/users";
import { RouteConfig } from "@/router";
import { LocalData } from "@/utils/localData";
import { AuthHttpApi } from "@/api/auth";
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from "@/utils/constants/actionNames";
import { PROJECTS_ACTIONS } from "@/store/modules/projects";
import { USER_ACTIONS } from "@/store/modules/users";
import { ACCESS_GRANTS_ACTIONS } from "@/store/modules/accessGrants";
import { BUCKET_ACTIONS } from "@/store/modules/buckets";
import { OBJECTS_ACTIONS } from "@/store/modules/objects";
import { PAYMENTS_MUTATIONS } from "@/store/modules/payments";

import SettingsIcon from '@/../static/images/navigation/settings.svg';
import BillingIcon from '@/../static/images/navigation/billing.svg';
import PlanIcon from '@/../static/images/navigation/plan.svg';
import LogoutIcon from '@/../static/images/navigation/logout.svg';

// @vue/component
@Component({
    components: {
        SettingsIcon,
        BillingIcon,
        PlanIcon,
        LogoutIcon,
    }
})
export default class AccountArea extends Vue {
    private readonly auth: AuthHttpApi = new AuthHttpApi();
    private dropdownYPos = 0;
    private dropdownXPos = 0;
    private dropdownWidth = 0;

    public $refs!: {
        accountArea: HTMLDivElement,
    }

    /**
     * Navigates user to account settings page.
     */
    public navigateToSettings(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Settings).path)
    }

    /**
     * Navigates user to account billing page.
     */
    public navigateToBilling(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).path)
    }

    /**
     * Toggles upgrade account popup.
     */
    public onUpgrade(): void {
        this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }

    /**
     * Logouts user and navigates to login page.
     */
    public async onLogout(): Promise<void> {
        await this.$router.push(RouteConfig.Login.path);

        try {
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
        const DROPDOWN_HEIGHT = 262;
        const accountContainer = this.$refs.accountArea.getBoundingClientRect();

        this.dropdownYPos = accountContainer.top - DROPDOWN_HEIGHT;
        this.dropdownXPos = accountContainer.left;
        this.dropdownWidth = accountContainer.width;

        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
    }

    /**
     * Closes dropdowns.
     */
    public closeDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }

    /**
     * Returns bottom and left position of dropdown.
     */
    public get style(): Record<string, string> {
        return { top: `${this.dropdownYPos}px`, left: `${this.dropdownXPos}px`, width: `${this.dropdownWidth}px` };
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

    /**
     * Returns first letter of user`s name.
     */
    public get avatarLetter(): string {
        return this.$store.getters.userName.slice(0, 1).toUpperCase();
    }
}
</script>

<style scoped lang="scss">
    .account-area {
        margin: 32px 20px;
        padding: 8px;
        width: calc(100% - 56px);
        border-radius: 0 0 10px 10px;
        display: flex;
        align-items: center;
        cursor: pointer;
        position: static;

        &__avatar {
            width: 40px;
            height: 40px;
            border-radius: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #2582ff;
            cursor: pointer;

            &__letter {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #fff;
            }
        }

        &__info {
            margin-left: 16px;

            &__email {
                font-family: 'font_bold', sans-serif;
                font-size: 14px;
                line-height: 20px;
                color: #56606d;
                white-space: nowrap;
                text-overflow: ellipsis;
                overflow: hidden;
            }

            &__type {
                font-size: 14px;
                line-height: 20px;
                color: #56606d;
            }
        }

        &__dropdown {
            position: absolute;
            background: #fff;
            min-width: 230px;
            z-index: 1;
            cursor: default;
            border: 1px solid #ebeef1;
            box-sizing: border-box;
            box-shadow: 0 -2px 16px rgba(0, 0, 0, 0.1);
            border-radius: 8px 8px 0 0;

            &__header {
                padding: 16px;
                width: calc(100% - 32px);
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__greet {
                    font-family: 'font_bold', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
                }

                &__sat {
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
                }
            }

            &__item {
                display: flex;
                align-items: center;
                padding: 16px;
                width: calc(100% - 32px);
                cursor: pointer;

                &__label {
                    margin-left: 16px;
                    font-size: 14px;
                    line-height: 20px;
                    color: #56606d;
                }

                &:hover {
                    background-color: #f7f8fb;
                }
            }
        }
    }

    .active {
        background-color: #f7f8fb;
    }

    @media screen and (max-width: 1280px) {

        .account-area__info {
            display: none;
        }
    }
</style>
