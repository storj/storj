// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./ProjectCreationSuccessPopup.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import ProjectCreationSuccessIcon from '@/../static/images/project/projectCreationSuccess.svg';

import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        VButton,
        ProjectCreationSuccessIcon,
        CloseCrossIcon,
    },
})
export default class ProjectCreationSuccessPopup extends Vue {
    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP);
    }

    /**
     * Changes location to api keys route.
     */
    public onCreateAPIKeyClick(): void {
        this.$router.push(RouteConfig.ApiKeys.path);
        this.onCloseClick();
    }

    /**
     * Indicates if component should be rendered.
     */
    public get isPopupShown(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulProjectCreationPopupShown;
    }
}
</script>

<style src="./ProjectCreationSuccessPopup.scss" scoped lang="scss"></style>
