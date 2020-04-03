// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="referral-container" :class="{ collapsed: isBannerShown }">
        <div class="referral-container__title-container">
            <p class="referral-container__title-container__text">Refer A Friend And Help Build The</p>
            <p class="referral-container__title-container__text">Decentralized Future</p>
        </div>
        <div class="referral-container__available" v-if="isAvailableLinks">
            <p class="referral-container__available__title">You Have {{ referralLinks.length }} Invitations To Share!</p>
            <div
                class="referral-container__copy-and-share-container__link-holder"
                v-for="link in referralLinks"
                :key="link.url"
            >
                <p class="referral-container__copy-and-share-container__link-holder__link">{{ link.url }}</p>
                <div class="copy-button" v-clipboard:copy="link.url" @click="copyLink">Copy</div>
            </div>
        </div>
        <div class="referral-container__not-available" v-else>
            <p class="referral-container__not-available__text">No available referral links. Try again later.</p>
            <NoLinksIcon />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NoLinksIcon from '@/../static/images/referral/NoLinks.svg';

import { REFERRAL_ACTIONS } from '@/store/modules/referral';
import { ReferralLink } from '@/types/referral';

@Component({
    components: {
        NoLinksIcon,
    },
})
export default class ReferralArea extends Vue {
    public copyLink(): void {
        this.$notify.success('Link saved to clipboard');
    }

    public get isBannerShown(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }

    public get isAvailableLinks(): boolean {
        return this.$store.state.referralModule.referralTokens.length !== 0;
    }

    public get referralLinks(): ReferralLink[] {
        return this.$store.getters.referralLinks;
    }

    public async beforeMount() {
        await this.$store.dispatch(REFERRAL_ACTIONS.GET_TOKENS);
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar {
        width: 0;
    }

    .referral-container {
        position: relative;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        height: 100%;
        padding: 50px 0 100px 0;

        &__title-container {
            display: flex;
            flex-direction: column;
            justify-items: center;
            align-items: center;

            &__text {
                text-align: center;
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 40px;
                color: #384b65;
                margin: 0;
            }
        }

        &__available {
            margin-top: 60px;
            padding-bottom: 75px;

            &__title {
                text-align: center;
                font-family: 'font_medium', sans-serif;
                font-size: 26px;
                color: #354049;
            }
        }

        &__copy-and-share-container {
            background-color: #fff;
            margin-top: 40px;
            padding: 40px 136px 40px 136px;
            border-radius: 24px;

            &__link-holder {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 10px 15px 10px 21px;
                background-color: white;
                border-radius: 6px;
                margin-top: 24px;

                &__link {
                    font-size: 16px;
                    line-height: 134%;
                    color: #494949;
                    margin-right: 10px;
                }

                .copy-button {
                    width: 140px;
                    height: 46px;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    background-color: #2683ff;
                    color: #fff;
                    font-family: 'font_regular', sans-serif;
                    border-radius: 6px;
                    font-weight: 900;
                    font-size: 16px;
                    line-height: 23px;

                    &:hover {
                        box-shadow: 0 4px 14px rgba(38, 131, 255, 0.3);
                        cursor: pointer;
                    }
                }
            }
        }

        &__not-available {

            &__text {
                margin: 30px 0 60px 0;
                text-align: center;
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                color: #384b65;
            }
        }
    }

    .collapsed {
        height: auto !important;
    }
</style>
