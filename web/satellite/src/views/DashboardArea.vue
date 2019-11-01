// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-container">
        <div v-if="isLoading" class="loading-overlay active">
            <img class="loading-image" src="@/../static/images/register/Loading.gif" alt="Company logo loading gif">
        </div>
        <div v-if="!isLoading" class="dashboard-container__wrap">
            <NavigationArea class="regular-navigation"/>
            <div class="dashboard-container__wrap__column">
                <DashboardHeader/>
                <div class="dashboard-container__main-area">
                    <router-view/>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DashboardHeader from '@/components/header/HeaderArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { PROJECT_USAGE_ACTIONS } from '@/store/modules/usage';
import { USER_ACTIONS } from '@/store/modules/users';
import { Project } from '@/types/projects';
import { AuthToken } from '@/utils/authToken';
import {
    API_KEYS_ACTIONS,
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';

const {
    SETUP_ACCOUNT,
    GET_BALANCE,
    GET_CREDIT_CARDS,
    GET_BILLING_HISTORY,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        NavigationArea,
        DashboardHeader,
    },
})
export default class DashboardArea extends Vue {
    public async mounted(): Promise<void> {
        // TODO: combine all project related requests in one
        try {
            await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
            await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
            await this.$notify.error(error.message);
            AuthToken.remove();
            await this.$router.push(RouteConfig.Login.path);

            return;
        }

        // try {
        //     await this.$store.dispatch(SETUP_ACCOUNT);
        //     await this.$store.dispatch(GET_BALANCE);
        //     await this.$store.dispatch(GET_CREDIT_CARDS);
        //     await this.$store.dispatch(GET_BILLING_HISTORY);
        // } catch (error) {
        //     if (error instanceof ErrorUnauthorized) {
        //         AuthToken.remove();
        //         await this.$router.push(RouteConfig.Login.path);
        //
        //         return;
        //     }
        //
        //    await this.$notify.error(error.message);
        // }

        let projects: Project[] = [];

        try {
            projects = await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        if (!projects.length) {
            await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED_EMPTY);

            if (!this.isRouteAccessibleWithoutProject()) {
                try {
                    await this.$router.push(RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path);
                } catch (err) {
                    return;
                }
            }

            return;
        }

        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projects[0].id);

        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        try {
            await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch api keys. ${error.message}`);
        }

        try {
            await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project usage. ${error.message}`);
        }

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`);
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
    }

    public get isLoading(): boolean {
        return this.$store.state.appStateModule.appState.fetchState === AppState.LOADING;
    }

    /**
     * This method checks if current route is available when user has no created projects
     */
    private isRouteAccessibleWithoutProject(): boolean {
        const awailableRoutes = [
            RouteConfig.Account.with(RouteConfig.Billing).path,
            RouteConfig.Account.with(RouteConfig.Profile).path,
            RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path,
        ];

        return awailableRoutes.includes(this.$router.currentRoute.path.toLowerCase());
    }
}
</script>

<style scoped lang="scss">
    .dashboard-container {
        position: fixed;
        max-width: 100%;
        width: 100%;
        height: 100%;
        left: 0;
        top: 0;
        background-color: #f5f6fa;
        z-index: 10;

        &__wrap {
            display: flex;

            &__column {
                display: flex;
                flex-direction: column;
                width: 100%;
            }
        }

        &__main-area {
            position: relative;
            width: 100%;
            height: 100%;
        }
    }

    @media screen and (max-width: 1024px) {

        .regular-navigation {
            display: none;
        }
    }

    @media screen and (max-width: 720px) {

        .dashboard-container {

            &__main-area {
                left: 60px;
            }
        }
    }

    .loading-overlay {
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        height: 100vh;
        z-index: 100;
        background-color: rgba(134, 134, 148, 1);
        visibility: hidden;
        opacity: 0;
        -webkit-transition: all 0.5s linear;
        -moz-transition: all 0.5s linear;
        -o-transition: all 0.5s linear;
        transition: all 0.5s linear;

        .loading-image {
            z-index: 200;
        }
    }

    .loading-overlay.active {
        visibility: visible;
        opacity: 1;
    }
</style>
