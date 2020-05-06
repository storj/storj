// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-dropdown-choice-container" id="accountDropdown">
        <div class="account-dropdown-overflow-container">
            <div class="account-dropdown-item-container settings" v-if="!isOnboardingTour" @click="onAccountSettingsClick">
                <div class="account-dropdown-item-container__image-container">
                    <AccountSettingsIcon class="account-dropdown-item-container__image-container__image"/>
                </div>
                <h2 class="account-dropdown-item-container__title">Account Settings</h2>
            </div>
            <div class="account-dropdown-item-container logout" @click="onLogoutClick">
                <div class="account-dropdown-item-container__image-container">
                    <LogoutIcon class="account-dropdown-item-container__image-container__image"/>
                </div>
                <h2 class="account-dropdown-item-container__title">Log Out</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AccountSettingsIcon from '@/../static/images/header/accountSettings.svg';
import LogoutIcon from '@/../static/images/header/logout.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import {
    API_KEYS_ACTIONS,
    APP_STATE_ACTIONS,
    NOTIFICATION_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';

@Component({
    components: {
        AccountSettingsIcon,
        LogoutIcon,
    },
})
export default class AccountDropdown extends Vue {
    private readonly auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Closes account dropdown.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
    }

    /**
     * Changes location to account root route.
     */
    public onAccountSettingsClick(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Settings).path);
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
    }

    /**
     * Performs logout on backend than clears all user information from store and local storage.
     */
    public async onLogoutClick(): Promise<void> {
        try {
            await this.auth.logout();
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        this.$router.push(RouteConfig.Login.path);
        this.$store.dispatch(PM_ACTIONS.CLEAR);
        this.$store.dispatch(PROJECTS_ACTIONS.CLEAR);
        this.$store.dispatch(USER_ACTIONS.CLEAR);
        this.$store.dispatch(API_KEYS_ACTIONS.CLEAR);
        this.$store.dispatch(NOTIFICATION_ACTIONS.CLEAR);
        this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);

        LocalData.removeUserId();
    }
}
</script>

<style scoped lang="scss">
    .account-dropdown-choice-container {
        position: absolute;
        top: 75px;
        right: 0;
        border-radius: 4px;
        padding: 10px 0;
        box-shadow: 0 4px rgba(231, 232, 238, 0.6);
        background-color: #fff;
        z-index: 1120;
        border-top: 1px solid rgba(169, 181, 193, 0.3);
    }

    .account-dropdown-overflow-container {
        position: relative;
        width: 210px;
        height: auto;
        background-color: #fff;
    }

    .account-dropdown-item-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 20px;
        padding-right: 20px;

        &__title {
            font-family: 'font_regular', sans-serif;
            margin-left: 20px;
            font-size: 14px;
            line-height: 20px;
            color: #354049;
        }

        &:hover {
            background-color: #f2f2f6;

            .account-dropdown-svg-path {
                fill: #2683ff !important;
            }
        }

        &__image-container {
            width: 20px;

            &__image {
                object-fit: cover;
            }
        }
    }
</style>
