// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isBannerShowing" class="notification-wrap">
        <div class="notification-wrap__content">
            <div class="notification-wrap__content__left">
                <SunnyIcon class="notification-wrap__content__left__icon" />
                <p>Ready to upgrade? Upload up to 75TB and pay what you use only, no minimum. 150GB free included.</p>
            </div>
            <div class="notification-wrap__content__right">
                <a @click="openBanner">Upgrade Now</a>
                <CloseIcon class="notification-wrap__content__right__close" @click="onCloseClick" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AB_TESTING_ACTIONS } from '@/store/modules/abTesting';
import { ABHitAction } from '@/types/abtesting';

import SunnyIcon from '@/../static/images/notifications/sunnyicon.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

// @vue/component
@Component({
    components: {
        CloseIcon,
        SunnyIcon,
    },
})
export default class UpgradeNotification extends Vue {
    @Prop({ default: () => () => false })
    public readonly openAddPMModal: () => void;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public isBannerShowing = true;

    /**
     * Closes notification.
     */
    public onCloseClick(): void {
        this.isBannerShowing = false;
    }

    // Send analytics event to segment when Upgrade Account banner is clicked.
    public async openBanner(): Promise<void> {
        this.openAddPMModal();
        await this.analytics.eventTriggered(AnalyticsEvent.UPGRADE_BANNER_CLICKED);
        await this.$store.dispatch(AB_TESTING_ACTIONS.HIT, ABHitAction.UPGRADE_ACCOUNT_CLICKED);
    }
}
</script>

<style scoped lang="scss">
    .notification-wrap {
        position: relative;

        &__content {
            position: absolute;
            left: 40px;
            right: 44px;
            top: 5px;
            z-index: 9999;
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            background-color: #fff;
            border: 1px solid #d8dee3;
            border-radius: 16px;
            box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

            &__left {
                display: flex;
                align-items: center;

                & b {
                    font-family: 'font_medium', sans-serif;
                }

                &__icon {
                    flex-shrink: 0;
                    margin-right: 16px;
                }
            }

            &__right {
                display: flex;
                align-items: center;
                flex-shrink: 0;
                margin-left: 16px;

                & a {
                    color: black;
                    text-decoration: underline !important;
                }

                &__close {
                    margin-left: 16px;
                    cursor: pointer;
                }
            }
        }
    }
</style>
