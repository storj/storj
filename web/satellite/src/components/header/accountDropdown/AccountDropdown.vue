// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-dropdown">
        <div class="account-dropdown__wrap">
            <div class="account-dropdown__wrap__name-container">
                <p class="account-dropdown__wrap__name-container__name">{{ userName }}</p>
            </div>
            <div class="account-dropdown__wrap__item-container" @click.stop="onLogoutClick">
                <p class="account-dropdown__wrap__item-container__title">Sign Out</p>
                <p class="account-dropdown__wrap__item-container__arrow">-></p>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

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

@Component
export default class AccountDropdown extends Vue {
    private readonly auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Returns user's full name.
     */
    public get userName(): string {
        return this.$store.getters.userName;
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
}
</script>

<style scoped lang="scss">
    .account-dropdown {
        position: absolute;
        top: 65px;
        right: 0;
        padding: 6px 0;
        background-color: #f5f6fa;
        z-index: 1120;
        box-shadow: 0 20px 34px 0 rgba(10, 27, 44, 0.28);
        border-radius: 6px;

        &__wrap {
            font-family: 'font_regular', sans-serif;
            min-width: 250px;
            max-width: 250px;
            background-color: #f5f6fa;
            border-radius: 6px;

            &__name-container {
                width: calc(100% - 45px);
                margin-left: 30px;
                padding: 0 15px 15px 0;

                &__name {
                    font-size: 14px;
                    line-height: 19px;
                    color: #1b2533;
                    white-space: nowrap;
                    overflow: hidden;
                    text-overflow: ellipsis;
                    margin: 15px 0 0 0;
                }
            }

            &__item-container {
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 20px 20px 20px 30px;
                cursor: pointer;

                &__title,
                &__arrow {
                    font-size: 14px;
                    line-height: 19px;
                    color: #0068dc;
                    font-weight: 500;
                    margin: 0;
                }
            }
        }
    }
</style>
