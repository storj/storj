// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./navigationArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelectionArea from '@/components/header/projectSelection/ProjectSelectionArea.vue';

import ApiKeysIcon from '@/../static/images/navigation/apiKeys.svg';
import DashboardIcon from '@/../static/images/navigation/dashboard.svg';
import DocsIcon from '@/../static/images/navigation/docs.svg';
import LogoIcon from '@/../static/images/navigation/logo.svg';
import LogoTextIcon from '@/../static/images/navigation/logoText.svg';
import SupportIcon from '@/../static/images/navigation/support.svg';
import TeamIcon from '@/../static/images/navigation/team.svg';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        ProjectSelectionArea,
        LogoIcon,
        LogoTextIcon,
        DocsIcon,
        SupportIcon,
        DashboardIcon,
        ApiKeysIcon,
        TeamIcon,
    },
})
export default class NavigationArea extends Vue {
    /**
     * Array of navigation links with icons.
     */
    public readonly navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(DashboardIcon),
        RouteConfig.ApiKeys.withIcon(ApiKeysIcon),
        RouteConfig.Team.withIcon(TeamIcon),
    ];

    /**
     * Array of account related navigation links.
     */
    public readonly accountNavigation: NavigationLink[] = [
        RouteConfig.Account.with(RouteConfig.Settings),
        RouteConfig.Account.with(RouteConfig.Billing),
        // TODO: disabled until implementation
        // RouteConfig.Account.with(RouteConfig.Referral),
    ];

    /**
     * Returns home path depending on app's state.
     */
    public get homePath(): string {
        if (this.isOnboardingTour) {
            return RouteConfig.OnboardingTour.path;
        }

        return RouteConfig.ProjectDashboard.path;
    }

    /**
     * Indicates if roter link is disabled.
     */
    public get isLinkDisabled(): boolean {
        return this.isOnboardingTour || this.isNoProject;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    private get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }

    /**
     * Indicates if there is no projects.
     */
    private get isNoProject(): boolean {
        return this.$store.state.projectsModule.projects.length === 0;
    }
}
</script>

<style src="./navigationArea.scss" lang="scss"></style>
