// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import ROUTES from '@/utils/constants/routerConstants';
import Login from '@/views/Login.vue';
import Register from '@/views/Register.vue';
import Dashboard from '@/views/Dashboard.vue';
import AccountArea from '@/components/dashboard/account/AccountArea.vue';
import ProjectDetails from '@/components/projectDetails/ProjectDetailsArea.vue';
import TeamArea from '@/components/team/TeamArea.vue';
import { getToken } from "@/utils/tokenManager";

Vue.use(Router);

let router = new Router({
	mode: 'history',
	routes: [
		{
			path: ROUTES.LOGIN.path,
			name: ROUTES.LOGIN.name,
			component: Login
		},
		{
			path: ROUTES.REGISTER.path,
			name: ROUTES.REGISTER.name,
			component: Register
		},
		{
			path: ROUTES.DASHBOARD.path,
			name: ROUTES.DASHBOARD.name,
			component: Dashboard,
			children: [
				{
					path: '/account-settings',
					name: 'AccountSettings',
					component: AccountArea
				},
				{
					path: '/project-details',
					name: 'ProjectDetails',
					component: ProjectDetails
				},
				{
					path: '/team',
					name: 'Team',
					component: TeamArea
				}
			]
		}
  	]
});

// Makes check that Token exist at session storage before any route except Login and Register
router.beforeEach((to, from, next) => {

    if(to.matched.some(record => record.meta.requiresAuth)) {
        if (!getToken()) {

            next(ROUTES.LOGIN)
            return;
        }
    }

    next();
});

export default router;

