// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-container">
        <DashboardHeader />
        <div class="dashboard-container__wrap">
            <NavigationArea />
            <div class="dashboard-container__main-area">
                <router-view />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import DashboardHeader from '@/components/header/Header.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import { removeToken, setToken } from '@/utils/tokenManager';
import { NOTIFICATION_ACTIONS, PROJETS_ACTIONS, PM_ACTIONS, USER_ACTIONS } from '@/utils/constants/actionNames';
import ROUTES from '@/utils/constants/routerConstants';

@Component({
    beforeMount: async function() {
    	const activationTokenParam = this.$route.query['activationToken'];

    	if(activationTokenParam) {
			const response = await this.$store.dispatch(USER_ACTIONS.ACTIVATE, activationTokenParam);
			if(!response.isSuccess) {
				this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to activate account');
				this.$router.push(ROUTES.LOGIN);

				removeToken();

				return;
            }

			setToken(response.data);
        }
        // TODO: should place here some animation while all needed data is fetching
        let response: RequestResponse<User> = await this.$store.dispatch(USER_ACTIONS.GET);

        if (!response.isSuccess) {
            this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);
            this.$router.push(ROUTES.LOGIN);
            removeToken();

            return;
        }

        let getProjectsResponse: RequestResponse<Project[]> = await this.$store.dispatch(PROJETS_ACTIONS.FETCH);

        if (!getProjectsResponse.isSuccess || getProjectsResponse.data.length < 1) {

            return;
        }

        this.$store.dispatch(PROJETS_ACTIONS.SELECT, getProjectsResponse.data[0].id);

        if (!this.$store.getters.selectedProject.id) return;

        const projectMembersResponse = await this.$store.dispatch(PM_ACTIONS.FETCH, {limit: 20, offset: 0});

        if (projectMembersResponse.isSuccess) return;

        this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, 'Unable to fetch project members');
    },
    components: {
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
</style>