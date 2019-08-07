// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="short-info-popup" v-if="isNotificationShown">
        <p>{{daysRemaining}} Days and ${{creditsRemaining}} Credits Remaining. Billing begins after expiration. <a>Learn more</a></p>
        <div @click="hideNotification">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M11 1L1 11M1 1L11 11" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
        </div>
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import { CREDIT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
    import { StorageManager } from '@/utils/storageManager';

    @Component
    export default class ReferralNotification extends Vue {
        public isReferralNotificationHidden: boolean = false;

        public get daysRemaining(): number {
            return this.$store.state.creditUsageModule.referralInfo.awardCreditDurationDays;
        }

        public get creditsRemaining(): string {
            const cents = this.$store.state.creditUsageModule.referralInfo.awardCreditInCent;
            let resultString = cents < 100 ? `0.${cents}` : `${cents / 100}`;

            return resultString;
        }

        public get isNotificationShown(): boolean {
            const isDataExist = !!this.daysRemaining && !!this.creditsRemaining;
            const isHiddenByFlags = !this.isReferralNotificationHidden && !StorageManager.isReferralNotificationHidden;

            return isDataExist && isHiddenByFlags;
        }

        public hideNotification() {
            StorageManager.setIsReferralNotificationHidden();
            this.isReferralNotificationHidden = true;
        }

        public async mounted() {
            await this.$store.dispatch(CREDIT_USAGE_ACTIONS.FETCH_REFERRAL_INFO);
        }
    }
</script>

<style scoped lang="scss">
    .short-info-popup {
        display: flex;
        flex-direction: row;
        position: absolute;
        top: 0;
        left: 0;
        height: 32px;
        width: calc(100% - 90px);
        background-color: #2683FF;
        justify-content: space-between;
        align-items: center;
        padding-left: 60px;
        padding-right: 30px;
        z-index: 98;

        p {
            font-family: 'font_regular';
            font-size: 13px;
            color: #fff;
        }
    }
</style>
