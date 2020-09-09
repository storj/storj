// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./resourcesDropdown.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CommunityIcon from '@/../static/images/header/community.svg';
import DocsIcon from '@/../static/images/header/docs.svg';
import SupportIcon from '@/../static/images/header/support.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        DocsIcon,
        CommunityIcon,
        SupportIcon,
    },
})
export default class ResourcesDropdown extends Vue {
    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.name === RouteConfig.OnboardingTour.name;
    }
}
</script>

<style src="./resourcesDropdown.scss" scoped lang="scss"></style>
