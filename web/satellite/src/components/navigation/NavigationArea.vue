// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./navigationArea.html"></template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import NAVIGATION_ITEMS from '@/utils/constants/navigationLinks';
    import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
    import AddUserPopup from '@/components/team/AddUserPopup.vue';

    @Component({
        components: {
            AddUserPopup,
        }
    })
    export default class NavigationArea extends Vue {
        // TODO: create types for navigation items
        public readonly navigation: any = NAVIGATION_ITEMS;

        public togglePopup(): void {
            if (!this.$store.getters.selectedProject.id) return;

            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS);
        }

        public onLogoClick(): void {
            location.reload();
        }

        public get isAddTeamMembersPopupShown(): boolean {
            return this.$store.state.appStateModule.appState.isAddTeamMembersPopupShown;
        }

        public get isProjectNotSelected(): boolean {
            return this.$store.state.projectsModule.selectedProject.id === '';
        }
    }
</script>

<style src="./navigationArea.scss" lang="scss"></style>
