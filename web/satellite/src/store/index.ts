// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Vuex from 'vuex';

import { authModule } from "@/store/modules/users";
import { projectsModule } from "@/store/modules/projects";
import { notificationsModule } from '@/store/modules/notifications';

Vue.use(Vuex);

// Satellite store (vuex)
const store = new Vuex.Store({
	modules: {
	    authModule,
		projectsModule,
		notificationsModule,
	},
});
  
export default store;
