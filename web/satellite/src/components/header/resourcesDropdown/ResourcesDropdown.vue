// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./resourcesDropdown.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CommunityIcon from '@/../static/images/header/community.svg';
import DocsIcon from '@/../static/images/header/docs.svg';
import SupportIcon from '@/../static/images/header/support.svg';

import { RouteConfig } from '@/router';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';

@Component({
    components: {
        DocsIcon,
        CommunityIcon,
        SupportIcon,
    },
})
export default class ResourcesDropdown extends Vue {
    
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    public onDocsIconClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, "https://documentation.storj.io");
    }

    public onCommunityIconClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, "https://storj.io/community/");
    }

    public onSupportIconClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, "mailto:support@storj.io");
    }
}
</script>

<style src="./resourcesDropdown.scss" scoped lang="scss"></style>
