// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./navigationArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelectionArea from '@/components/header/projectSelection/ProjectSelectionArea.vue';

import DocsIcon from '@/../static/images/navigation/docs.svg';
import LogoIcon from '@/../static/images/navigation/logo.svg';
import LogoTextIcon from '@/../static/images/navigation/logoText.svg';
import SupportIcon from '@/../static/images/navigation/support.svg';

import { RouteConfig } from '@/router';
import { NavigationLink } from '@/types/navigation';

@Component({
    components: {
        ProjectSelectionArea,
        LogoIcon,
        LogoTextIcon,
        DocsIcon,
        SupportIcon,
    },
})
export default class NavigationArea extends Vue {
    /**
     * Indicates if resource related navigation links appears.
     */
    public areResourceItemsShown: boolean = true;
    public isResourceButtonShown: boolean = false;

    /**
     * Indicates if account related navigation links appears.
     */
    public areAccountItemsShown: boolean = true;
    public isAccountButtonShown: boolean = false;

    /**
     * Toggles resource items visibility.
     */
    public toggleResourceItemsVisibility(): void {
        this.areResourceItemsShown = !this.areResourceItemsShown;
    }

    /**
     * Toggles resource button visibility.
     */
    public toggleResourceButtonVisibility(): void {
        this.isResourceButtonShown = !this.isResourceButtonShown;
    }

    /**
     * Toggles account items visibility.
     */
    public toggleAccountItemsVisibility(): void {
        this.areAccountItemsShown = !this.areAccountItemsShown;
    }

    /**
     * Toggles account button visibility.
     */
    public toggleAccountButtonVisibility(): void {
        this.isAccountButtonShown = !this.isAccountButtonShown;
    }

    /**
     * Array of navigation links with icons.
     */
    // TODO: Use SvgLoaderComponent to reduce markup lines
    public readonly navigation: NavigationLink[] = [
        RouteConfig.ProjectDashboard.withIcon(
            `<svg class="svg" width="19" height="17" viewBox="0 0 19 17" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="navigation-svg-path" fill-rule="evenodd" clip-rule="evenodd" d="M15.6691 16.8889H9.5H3.3309C1.29229 15.1294 0 12.5137 0 9.59331C0 4.29507 4.2533 0 9.5 0C14.7467 0 19 4.29507 19 9.59331C19 12.5137 17.7077 15.1294 15.6691 16.8889ZM12.9227 5.32961L14.4156 6.83704L9.93721 11.3594L8.44444 9.85192L12.9227 5.32961Z" fill="#7C8794"/>
              </svg>
             `),
        RouteConfig.Team.withIcon(
            `<svg class="svg" width="21" height="18" viewBox="0 0 21 18" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="navigation-svg-path" opacity="0.8" fill-rule="evenodd" clip-rule="evenodd" d="M14.8165 0C17.7839 0 20.1895 2.40557 20.1895 5.373C20.1895 7.40389 19.0627 9.17161 17.4002 10.0851C19.4392 11.0968 20.8407 13.2 20.8407 15.6305V17.91H8.46655V15.6305C8.46655 13.13 9.94989 10.976 12.0846 10.0004C10.5035 9.06519 9.44345 7.34289 9.44345 5.373C9.44345 2.40557 11.849 0 14.8165 0ZM6.34991 0C7.36246 0 8.3096 0.280088 9.11806 0.767013C8.09966 2.02549 7.48964 3.62801 7.48964 5.373L7.49091 5.51065C7.51986 7.06803 8.03908 8.53529 8.92906 9.73552L8.97519 9.79612L8.94777 9.82267C7.42471 11.3194 6.51273 13.3922 6.51273 15.6305V17.91H0V15.6305C0 13.13 1.48335 10.976 3.61809 10.0004C2.037 9.06519 0.976909 7.34289 0.976909 5.373C0.976909 2.40557 3.38248 0 6.34991 0Z" fill="#909BA8"/>
              </svg>
             `),
        RouteConfig.ApiKeys.withIcon(
            `<svg class="svg" width="19" height="19" viewBox="0 0 19 19" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path class="navigation-svg-path" opacity="0.8" fill-rule="evenodd" clip-rule="evenodd" d="M19 0L18.8777 3.30167L17.3804 3.33154L17.3114 4.86798L15.548 5.164L15.5552 6.62419L14.0737 6.63827L13.8958 8.28354L12.3349 9.84422L13.2397 13.2214C13.3996 13.8182 13.229 14.4549 12.7921 14.8918L9.19044 18.4934C8.75356 18.9303 8.1168 19.1009 7.52001 18.941L2.60009 17.6227C2.00331 17.4628 1.53716 16.9967 1.37725 16.3999L0.0589658 11.48C-0.100943 10.8832 0.0696777 10.2464 0.506556 9.80956L4.10819 6.20793C4.54507 5.77105 5.18183 5.60043 5.77862 5.76034L9.15552 6.66484L15.6983 0.122284L19 0ZM6.52703 12.473C5.78414 11.7301 4.57967 11.7301 3.83678 12.473C3.09389 13.2159 3.09389 14.4203 3.83678 15.1632C4.57967 15.9061 5.78414 15.9061 6.52703 15.1632C7.26992 14.4203 7.26992 13.2159 6.52703 12.473Z" fill="#909BA8"/>
              </svg>
             `),
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
     * Indicates if resources displaying button is shown.
     */
    public get isResourcesDisplayingButtonShown(): boolean {
        return !this.areResourceItemsShown && this.isResourceButtonShown;
    }

    /**
     * Indicates if resources items hiding button is shown.
     */
    public get isResourcesHidingButtonShown(): boolean {
        return this.areResourceItemsShown && this.isResourceButtonShown;
    }

    /**
     * Indicates if account items displaying button is shown.
     */
    public get isAccountItemsDisplayingButtonShown(): boolean {
        return !this.areAccountItemsShown && this.isAccountButtonShown;
    }

    /**
     * Indicates if account items hiding button is shown.
     */
    public get isAccountItemsHidingButtonShown(): boolean {
        return this.areAccountItemsShown && this.isAccountButtonShown;
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
