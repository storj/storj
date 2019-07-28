import {AppState} from "../utils/constants/appStateEnum";
// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-container">
        <div v-if="isLoading" class="loading-overlay active">
            <img src="../../static/images/register/Loading.gif">
        </div>
        <div class="dashboard-container__wrap">
            <NavigationArea />
            <div class="dashboard-container__wrap__column">
                <DashboardHeader />
                <div class="dashboard-container__main-area">
                    <router-view />
                </div>
            </div>
        </div>
        <ProjectCreationSuccessPopup/>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import DashboardHeader from '@/components/header/Header.vue';
    import NavigationArea from '@/components/navigation/NavigationArea.vue';
    import { AuthToken } from '@/utils/authToken';
    import {
        API_KEYS_ACTIONS,
        APP_STATE_ACTIONS,
        NOTIFICATION_ACTIONS,
        PM_ACTIONS,
        PROJETS_ACTIONS,
        USER_ACTIONS,
        PROJECT_USAGE_ACTIONS,
        BUCKET_USAGE_ACTIONS, PROJECT_PAYMENT_METHODS_ACTIONS
    } from '@/utils/constants/actionNames';
    import ROUTES from '@/utils/constants/routerConstants';
    import ProjectCreationSuccessPopup from '@/components/project/ProjectCreationSuccessPopup.vue';
    import { AppState } from '../utils/constants/appStateEnum';
    import { RequestResponse } from '../types/response';
    import { User } from '../types/users';
    import { Project } from '@/types/projects';

    @Component({
    mounted: async function() {
        setTimeout(async () => {
            let response: RequestResponse<User> = await this.$store.dispatch(USER_ACTIONS.GET);
            if (!response.isSuccess) {
                this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
                this.$router.push(ROUTES.LOGIN);
                AuthToken.remove();

                return;
            }

            let getProjectsResponse: RequestResponse<Project[]> = await this.$store.dispatch(PROJETS_ACTIONS.FETCH);
            if (!getProjectsResponse.isSuccess || getProjectsResponse.data.length < 1) {
                this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED_EMPTY);

                return;
            }

            await this.$store.dispatch(PROJETS_ACTIONS.SELECT, getProjectsResponse.data[0].id);

            await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
            const projectMembersResponse = await this.$store.dispatch(PM_ACTIONS.FETCH);
            if (!projectMembersResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
            }

            const keysResponse = await this.$store.dispatch(API_KEYS_ACTIONS.FETCH);
            if (!keysResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch api keys');
            }

            const usageResponse = await this.$store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);
            if (!usageResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project usage');
            }

            const bucketsResponse = await this.$store.dispatch(BUCKET_USAGE_ACTIONS.FETCH, 1);
            if (!bucketsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch buckets: ' + bucketsResponse.errorMessage);
            }

            const paymentMethodsResponse = await this.$store.dispatch(PROJECT_PAYMENT_METHODS_ACTIONS.FETCH);
            if (!paymentMethodsResponse.isSuccess) {
                this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch payment methods: ' + paymentMethodsResponse.errorMessage);
            }

            this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
        }, 800);
    },
    computed: {
        isLoading: function() {
            return this.$store.state.appStateModule.appState.fetchState === AppState.LOADING;
        }
    },
    components: {
        ProjectCreationSuccessPopup,
        NavigationArea,
        DashboardHeader
    }
})
export default class Dashboard extends Vue {
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
        left: 0;
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
