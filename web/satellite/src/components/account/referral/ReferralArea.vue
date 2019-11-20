// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="referral-container">
        <div class="referral-container__wrapper">
            <div class="referral-container__copy-and-share-container">
                <p class="referral-container__copy-and-share-container__title">Copy And Share Your Invitation Link</p>
                <div class="referral-container__copy-and-share-container__link-holder">
                    <p class="referral-container__copy-and-share-container__link-holder__link">test...</p>
                    <div class="copy-button" v-clipboard="'test'" @click="copyLink">Copy</div>
                </div>
            </div>
            <div class="underline"></div>
            <p class="referral-container__help">Referral credits are only earned after your referral has used up their free credits. Credits will automatically be applied to your upcoming bill. There are unlimited invites, so invite as many people as you want. The invite you send is only for the Satellite you're connected to. If you have any questions please email
                <a href="mailto:support@tardigrade.io">support@tardigrade.io</a>
            </p>
        </div>
    </div>
</template>

<script lang="ts">
import VueClipboards from 'vue-clipboards';
import { Component, Vue } from 'vue-property-decorator';

Vue.use(VueClipboards);

@Component
export default class ReferralArea extends Vue {
    public copyLink(): void {
        this.$notify.success('Link saved to clipboard');
    }
}
</script>

<style scoped lang="scss">
    ::-webkit-scrollbar {
        width: 0;
    }

    .referral-container {
        position: relative;
        margin-top: 83px;
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        justify-items: center;
        align-items: center;

        &__wrapper {
            width: 966px;
            display: flex;
            justify-content: flex-start;
            flex-direction: column;
        }

        &__title {
            text-align: center;
            font-size: 32px;
            line-height: 33px;
            color: #354049;
            font-family: 'font_bold', sans-serif;
            margin-top: 80px;
        }

        &__subtitle {
            text-align: center;
            font-size: 22px;
            line-height: 24px;
            margin-top: 25px;
            color: #354049;
        }

        &__copy-and-share-container {
            background-color: #fff;
            margin-top: 40px;
            padding: 40px 136px 40px 136px;
            border-radius: 24px;

            &__title {
                font-size: 26px;
                line-height: 16px;
                color: #354049;
                text-align: center;
                font-family: 'font_regular', sans-serif;
                font-weight: 900;
                padding-bottom: 22px;
            }

            &__link-holder {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 16px 15px 16px 21px;
                background-color: #f5f6fa;
                border-radius: 6px;
                margin-bottom: 24px;

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

            &__share-links {
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;

                &__info {
                    color: rgba(116, 116, 116, 0.5);
                    font-size: 17px;
                    line-height: 16px;
                }
            }
        }

        &__help {
            text-align: center;
            max-width: 778px;
            font-size: 13px;
            line-height: 24px;
            color: #959595;
            margin: 40px 0 0 81px;
            justify-content: center;

            a {
                color: #00e;
                cursor: pointer;

                &:hover {
                    text-decoration: underline;
                }
            }
        }
    }
</style>
