// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./navigationArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { mapState } from 'vuex';
import NAVIGATION_ITEMS from '@/utils/constants/navigationLinks';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import AddUserPopup from '@/components/team/AddUserPopup.vue';

@Component(
    {
        data: function () {
            return {
                navigation: NAVIGATION_ITEMS,
                isPopupShown: false,
            };
        },
        components: {
            AddUserPopup,
        },
        methods: {
            togglePopup: function(): void {
                if (!this.$store.getters.selectedProject.id) return;

                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
            },
            onLogoClick: function (): void {
                location.reload();
            }
        },
        computed: mapState({
            isAddTeamMembersPopupShown: (state: any) => state.appStateModule.appState.isAddTeamMembersPopupShown,
        }),
    }
)

export default class NavigationArea extends Vue {
}
</script>

<style src="./navigationArea.scss" lang="scss"></style>
