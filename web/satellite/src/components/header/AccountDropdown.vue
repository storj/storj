// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-dropdown" id="accountDropdown">
        <div class="account-dropdown__link-container" v-if="!isOnboardingTour">
            <a
                class="account-dropdown__link-container__link"
                :href="projectLimitsIncreaseRequestURL"
                target="_blank"
                rel="noopener noreferrer"
            >
                Request Limit Increase
            </a>
        </div>
        <div class="account-dropdown__item-container settings" v-if="!isOnboardingTour" @click="onAccountSettingsClick">
            <div class="account-dropdown__item-container__image-container">
                <AccountSettingsIcon class="account-dropdown__item-container__image-container__image"/>
            </div>
            <p class="account-dropdown__item-container__title">Account Settings</p>
        </div>
        <div class="account-dropdown__item-container logout" @click="onLogoutClick">
            <p class="account-dropdown__item-container__title">Log Out</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AccountSettingsIcon from '@/../static/images/header/accountSettings.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { API_KEYS_ACTIONS } from '@/store/modules/apiKeys';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import {
    APP_STATE_ACTIONS,
    NOTIFICATION_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        AccountSettingsIcon,
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

        await this.$router.push(RouteConfig.Login.path);
        await this.$store.dispatch(PM_ACTIONS.CLEAR);
        await this.$store.dispatch(PROJECTS_ACTIONS.CLEAR);
        await this.$store.dispatch(USER_ACTIONS.CLEAR);
        await this.$store.dispatch(API_KEYS_ACTIONS.CLEAR);
        await this.$store.dispatch(NOTIFICATION_ACTIONS.CLEAR);
        await this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
        await this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);

        LocalData.removeUserId();
    }

    /**
     * Returns project limits increase request url from config.
     */
    public get projectLimitsIncreaseRequestURL(): string {
        return MetaUtils.getMetaContent('project-limits-increase-request-url');
    }
}
</script>

<style scoped lang="scss">
    .account-dropdown {
        position: absolute;
        top: 65px;
        right: 0;
        padding: 15px 0;
        background-color: #fff;
        z-index: 1120;
        font-family: 'font_regular', sans-serif;
        min-width: 210px;
        border: 1px solid #c5cbdb;
        box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
        border-radius: 6px;

        &__link-container {
            width: calc(100% - 20px);
            margin-left: 20px;
            border-bottom: 1px solid #c7cdd2;
            padding-bottom: 15px;

            &__link {
                font-size: 14px;
                line-height: 19px;
                color: #7e8b9c;
            }
        }

        &__item-container {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: flex-start;
            padding: 0 20px;

            &__title {
                margin-left: 13px;
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
                max-height: 20px;

                &__image {
                    object-fit: cover;
                }
            }
        }
    }

    .settings {
        margin-top: 15px;
    }

    .logout {
        padding: 0 20px 0 40px;
    }
</style>
