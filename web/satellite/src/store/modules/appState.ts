// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { APP_STATE_MUTATIONS } from '../mutationConstants';

export const appStateModule = {
	state: {
		// Object that contains all states of views
		currentAppState: {
            isAddTeamMembersPopupShown: false
        },
	},
	mutations: {
		// Mutation changing add team members popup state
		[APP_STATE_MUTATIONS.SET_ADD_TEAMMEMBER_POPUP_STATE](state: any, isShown: boolean): void {
            state.currentAppState.isAddTeamMembersPopupShown = isShown;
		},
		
	},
	actions: {
		// Commits muttation for changing add team members popup state
		setAddTeamMembersPopup: function ({commit}: any, isShown: boolean): void {
			commit(APP_STATE_MUTATIONS.SET_ADD_TEAMMEMBER_POPUP_STATE, isShown);
		},
	},
};
