// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="referral-container">
        <div class="referral-container__title-container">
            <p class="referral-container__title-container__text">Refer A Friend And Help Build The</p>
            <p class="referral-container__title-container__text">Decentralized Future</p>
        </div>
        <div v-if="true">
            <p class="referral-container__copy-and-share-container__link-title"></p>
            <div class="referral-container__copy-and-share-container__link-holder">
                <p class="referral-container__copy-and-share-container__link-holder__link">https://us-central-1.tardigrade.io/ref/?uuid=96a33796-2c9b-47</p>
                <div class="copy-button" v-clipboard="'test'" @click="copyLink">Copy</div>
            </div>
        </div>
        <p v-if="false">No available referral links. Try again later.</p>
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

        &__copy-and-share-container {
            background-color: #fff;
            margin-top: 40px;
            padding: 40px 136px 40px 136px;
            border-radius: 24px;

            &__link-holder {
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 16px 15px 16px 21px;
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
    }
</style>
