// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="pt-bar">
        <p>
            Upload up to 75TB by upgrading to a Storj Pro Account.
        </p>
        <p class="pt-bar__functional" @click="openBanner">
            Upgrade now.
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

// @vue/component
@Component
export default class PaidTierBar extends Vue {
    @Prop({ default: () => () => false })
    public readonly openAddPMModal: () => void;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    // Send analytics event to segment when Upgrade Account banner is clicked.
    public async openBanner(): Promise<void> {
        this.openAddPMModal();
        await this.analytics.eventTriggered(AnalyticsEvent.UPGRADE_BANNER_CLICKED);

    }
}
</script>

<style scoped lang="scss">
    .pt-bar {
        width: 100%;
        box-sizing: border-box;
        font-family: 'font_regular', sans-serif;
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: #0047ff;
        font-size: 14px;
        line-height: 18px;
        color: #eee;
        padding: 5px 30px;

        &__functional {
            font-family: 'font_bold', sans-serif;
            cursor: pointer;
        }
    }
</style>
