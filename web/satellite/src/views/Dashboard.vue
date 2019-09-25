// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-container">
        <div v-if="isLoading" class="loading-overlay active">
            <img src="../../static/images/register/Loading.gif">
        </div>
        <div v-if="!isLoading" class="dashboard-container__wrap">
            <NavigationArea class="regular-navigation" />
            <div class="dashboard-container__wrap__column">
                <DashboardHeader />
                <div class="dashboard-container__main-area">
                    <router-view />
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import DashboardHeader from '@/components/header/Header.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { PROJECT_USAGE_ACTIONS } from '@/store/modules/usage';
import { USER_ACTIONS } from '@/store/modules/users';
import { Project } from '@/types/projects';
import { AuthToken } from '@/utils/authToken';
import {
    API_KEYS_ACTIONS,
    APP_STATE_ACTIONS,
    NOTIFICATION_ACTIONS,
    PM_ACTIONS,
    PROJECT_PAYMENT_METHODS_ACTIONS,
} from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';

@Component({
    components: {
        NavigationArea,
        DashboardHeader,
    },
})
export default class Dashboard extends Vue {
    public mounted(): void {
        setTimeout(async () => {
            // TODO: combine all project related requests in one
            try {
                await this.$store.dispatch(USER_ACTIONS.GET);
            } catch (error) {
                await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
                await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);
                await this.$router.push(RouteConfig.Login.path);
                AuthToken.remove();

                return;
            }

            let projects: Project[] = [];

            try {
                projects = await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
            } catch (error) {
                await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, error.message);

                return;
            }

            if (!projects.length) {
                await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED_EMPTY);

                if (!this.isCurrentRouteIsAccount) {
                    await this.$router.push(RouteConfig.ProjectOverview.path);

                    return;
                }

                await this.$router.push(RouteConfig.ProjectOverview.path);
            }

            await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projects[0].id);

            await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
            try {
                await this.$store.dispatch(PM_ACTIONS.FETCH, 1);
            } catch (error) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch project members. ${error.message}`);
            }

            try {
                await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, 1);
            } catch (error) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch api keys. ${error.message}`);
            }

            try {
                await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
            } catch (error) {
                await this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, `Unable to fetch project usage. ${error.message}`);
            }

            try {
                await this.$store.dispatch(BUCKET_ACTIONS.FETCH, 1);
            } catch (error) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + error.message);
            }

            const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!paymentMethodsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch payment methods: ' + paymentMethodsResponse.errorMessage);
            }

            this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
        }, 800);
    }

    public get isLoading(): boolean {
        return this.$store.state.appStateModule.appState.fetchState === AppState.LOADING;
    }
    public get isCurrentRouteIsAccount(): boolean {
        const segments = this.$route.path.split('/').map(segment => segment.toLowerCase());

        return segments.includes(RouteConfig.Account.name.toLowerCase());
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
        background-color: #F5F6FA;
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

    @media screen and (max-width: 1024px)  {
        .regular-navigation {
            display: none;
        }
    }

    @media screen and (max-width: 720px) {
        .dashboard-container {
            &__main-area{
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

        img {
            z-index: 200;
        }
    }

    .loading-overlay.active {
        visibility: visible;
        opacity: 1;
    }
</style>
