// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="resources-dropdown">
        <a
            class="resources-dropdown__item-container"
            href="https://docs.storj.io/"
            target="_blank"
            rel="noopener noreferrer"
        >
            <DocsIcon class="resources-dropdown__item-container__image" />
            <p class="resources-dropdown__item-container__title">Docs</p>
        </a>
        <a
            class="resources-dropdown__item-container"
            href="https://forum.storj.io/"
            target="_blank"
            rel="noopener noreferrer"
        >
            <CommunityIcon class="resources-dropdown__item-container__image" />
            <p class="resources-dropdown__item-container__title">Community</p>
        </a>
        <a
            class="resources-dropdown__item-container"
            href="https://supportdcs.storj.io/hc/en-us"
            target="_blank"
            rel="noopener noreferrer"
        >
            <SupportIcon class="resources-dropdown__item-container__image" />
            <p class="resources-dropdown__item-container__title">Support</p>
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CommunityIcon from '@/../static/images/header/community.svg';
import DocsIcon from '@/../static/images/header/docs.svg';
import SupportIcon from '@/../static/images/header/support.svg';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

// @vue/component
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
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, 'https://docs.storj.io/node');
    }

    public onCommunityIconClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, 'https://storj.io/community/');
    }

    public onSupportIconClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.EXTERNAL_LINK_CLICKED, 'mailto:support@storj.io');
    }
}
</script>

<style scoped lang="scss">
    .resources-dropdown {
        position: absolute;
        top: 50px;
        left: 0;
        padding: 6px 0;
        background-color: #f5f6fa;
        z-index: 1120;
        font-family: 'font_regular', sans-serif;
        min-width: 255px;
        box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
        border-radius: 6px;

        &__item-container {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            padding: 0 20px;

            &__image {
                min-width: 20px;
                object-fit: cover;
            }

            &__title {
                margin: 8px 0 14px 13px;
                font-size: 14px;
                line-height: 20px;
                color: #1b2533;
            }

            &:hover {
                font-family: 'font_bold', sans-serif;

                .docs-svg-path,
                .community-svg-path,
                .support-svg-path {
                    fill: #2d75d2 !important;
                }
            }
        }
    }
</style>
