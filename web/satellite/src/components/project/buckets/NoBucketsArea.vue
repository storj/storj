// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="no-buckets-area">
        <img class="no-buckets-area__image" src="@/../static/images/buckets/bucket.png" alt="bucket image">
        <h2 class="no-buckets-area__message">Create your first bucket to get started.</h2>
        <VButton
            label="Get Started"
            width="156px"
            height="47px"
            :on-press="navigateToWelcomeScreen"
        />
        <a
            class="no-buckets-area__second-button"
            href="https://docs.storj.io/"
            target="_blank"
            rel="noopener noreferrer"
        >
            Visit the Docs
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';

// @vue/component
@Component({
    components: {
        VButton,
    },
})
export default class NoBucketArea extends Vue {

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Navigates user to welcome screen.
     */
    public navigateToWelcomeScreen(): void {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
        this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2 {
        margin: 0;
    }

    .no-buckets-area {
        padding: 60px 0;
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        border-radius: 6px;

        &__image {
            margin-right: 10px;
        }

        &__message {
            font-family: 'font_bold', sans-serif;
            font-size: 18px;
            line-height: 26px;
            color: #354049;
            margin: 15px 15px 30px;
            text-align: center;
        }

        &__second-button {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 156px;
            height: 47px;
            font-size: 15px;
            line-height: 22px;
            border-radius: 6px;
            background-color: #fff;
            color: #2683ff;
            border: 1px solid #2683ff;
            margin-top: 7px;

            &:hover {
                background-color: #2683ff;
                color: #fff;
            }
        }

        &__help {
            margin-top: 20px;
            font-size: 13px;
            line-height: 18px;
            text-decoration: underline;
            color: #2683ff;
        }
    }
</style>
