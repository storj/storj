// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="save-api-popup" v-if="isPopupShown">
        <h2>Name Your API Key</h2>
        <div class="save-api-popup__copy-area">
            <div class="save-api-popup__copy-area__key-area">
                <p>{{apiKeySecret}}</p>
            </div>
            <div class="copy-button" v-clipboard="apiKeySecret" @click="onCopyClick" v-if="!isCopiedButtonShown">
                <svg width="22" height="22" viewBox="0 0 22 22" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M13.3763 7.7002H3.34383C2.46383 7.7002 1.76001 8.40402 1.76001 9.28402V19.3378C1.76001 20.1954 2.46383 20.9216 3.34383 20.9216H13.3976C14.2553 20.9216 14.9814 20.2178 14.9814 19.3378L14.9823 9.28402C14.96 8.40402 14.2561 7.7002 13.3761 7.7002H13.3763ZM13.6401 19.3164C13.6401 19.4488 13.5301 19.5588 13.3977 19.5588L3.34397 19.5579C3.21162 19.5579 3.10161 19.4479 3.10161 19.3156L3.10247 9.284C3.10247 9.15165 3.21247 9.04164 3.34483 9.04164H13.3986C13.5309 9.04164 13.641 9.15164 13.641 9.284L13.6401 19.3164Z" fill="white"/>
                    <path d="M18.6563 1.09974H8.62386C7.74386 1.09974 7.04004 1.80356 7.04004 2.68356V6.37978H8.36004V2.68356C8.36004 2.55122 8.47004 2.44121 8.60239 2.44121H18.6562C18.7885 2.44121 18.8985 2.55121 18.8985 2.68356V12.7373C18.8985 12.8697 18.7885 12.9797 18.6562 12.9797H16.2799V14.2997H18.6562C19.5138 14.2997 20.24 13.5959 20.24 12.7159V2.68343C20.24 1.80343 19.5362 1.09961 18.6562 1.09961L18.6563 1.09974Z" fill="#354049"/>
                    <rect x="2.93335" y="8.7998" width="11.7333" height="11" fill="white"/>
                    <rect x="7.1001" y="1.2334" width="12.9333" height="12.9333" rx="1.5" fill="white" stroke="#2683FF"/>
                </svg>
                <p>Copy</p>
            </div>
            <div class="copied-button" v-if="isCopiedButtonShown">
                <svg width="22" height="22" viewBox="0 0 22 22" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M13.3763 7.7002H3.34383C2.46383 7.7002 1.76001 8.40402 1.76001 9.28402V19.3378C1.76001 20.1954 2.46383 20.9216 3.34383 20.9216H13.3976C14.2553 20.9216 14.9814 20.2178 14.9814 19.3378L14.9823 9.28402C14.96 8.40402 14.2561 7.7002 13.3761 7.7002H13.3763ZM13.6401 19.3164C13.6401 19.4488 13.5301 19.5588 13.3977 19.5588L3.34397 19.5579C3.21162 19.5579 3.10161 19.4479 3.10161 19.3156L3.10247 9.284C3.10247 9.15165 3.21247 9.04164 3.34483 9.04164H13.3986C13.5309 9.04164 13.641 9.15164 13.641 9.284L13.6401 19.3164Z" fill="white"/>
                    <path d="M18.6563 1.09974H8.62386C7.74386 1.09974 7.04004 1.80356 7.04004 2.68356V6.37978H8.36004V2.68356C8.36004 2.55122 8.47004 2.44121 8.60239 2.44121H18.6562C18.7885 2.44121 18.8985 2.55121 18.8985 2.68356V12.7373C18.8985 12.8697 18.7885 12.9797 18.6562 12.9797H16.2799V14.2997H18.6562C19.5138 14.2997 20.24 13.5959 20.24 12.7159V2.68343C20.24 1.80343 19.5362 1.09961 18.6562 1.09961L18.6563 1.09974Z" fill="#354049"/>
                    <rect x="2.93335" y="8.7998" width="11.7333" height="11" fill="white"/>
                    <rect x="7.1001" y="1.2334" width="12.9333" height="12.9333" rx="1.5" fill="white" stroke="#2683FF"/>
                </svg>
                <p>Copied</p>
            </div>
        </div>
        <div class="save-api-popup__close-cross-container" @click="onCloseClick">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path d="M15.7071 1.70711C16.0976 1.31658 16.0976 0.683417 15.7071 0.292893C15.3166 -0.0976311 14.6834 -0.0976311 14.2929 0.292893L15.7071 1.70711ZM0.292893 14.2929C-0.0976311 14.6834 -0.0976311 15.3166 0.292893 15.7071C0.683417 16.0976 1.31658 16.0976 1.70711 15.7071L0.292893 14.2929ZM1.70711 0.292893C1.31658 -0.0976311 0.683417 -0.0976311 0.292893 0.292893C-0.0976311 0.683417 -0.0976311 1.31658 0.292893 1.70711L1.70711 0.292893ZM14.2929 15.7071C14.6834 16.0976 15.3166 16.0976 15.7071 15.7071C16.0976 15.3166 16.0976 14.6834 15.7071 14.2929L14.2929 15.7071ZM14.2929 0.292893L0.292893 14.2929L1.70711 15.7071L15.7071 1.70711L14.2929 0.292893ZM0.292893 1.70711L14.2929 15.7071L15.7071 14.2929L1.70711 0.292893L0.292893 1.70711Z" fill="#384B65"/>
        </svg>
        </div>
        <div class="blur-content"></div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import Button from '@/components/common/Button.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        HeaderlessInput,
        Button,
    },
})
export default class ApiKeysCopyPopup extends Vue {
    @Prop({default: false})
    private readonly isPopupShown: boolean;
    @Prop({default: ''})
    private readonly apiKeySecret: string;

    private isCopiedButtonShown: boolean = false;

    public onCloseClick(): void {
        this.isCopiedButtonShown = false;
        this.$emit('closePopup');
    }

    public onCopyClick(): void {
        this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Key saved to clipboard');
        this.isCopiedButtonShown = true;
    }
}
</script>

<style scoped lang="scss">
    .save-api-popup {
        padding: 32px 40px 60px 40px;
        background-color: #FFFFFF;
        border-radius: 24px;
        margin-top: 29px;
        max-width: 94.8%;
        height: auto;
        position: relative;

        h2 {
            font-family: 'font_bold';
            font-size: 24px;
            line-height: 29px;
            margin-bottom: 26px;
        }

        &__copy-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            background-color: #F5F6FA;
            padding: 29px 32px 29px 24px;
            border-radius: 12px;
            position: relative;

            &__key-area {

                p {
                    font-family: 'font_regular';
                    margin: 0;
                    font-size: 16px;
                    line-height: 21px;
                    word-break: break-all;
                }
            }

            .copy-button,
            .copied-button {
                display: flex;
                background-color: #2683FF;
                padding: 13px 36px;
                cursor: pointer;
                align-items: center;
                justify-content: space-between;
                color: #FFFFFF;
                border: 1px solid #2683FF;
                box-sizing: border-box;
                border-radius: 8px;
                font-size: 14px;
                font-family: 'font_bold';
                margin-left: 10px;

                p {
                    margin: 0;
                }

                &:hover {
                    background-color: #196CDA;
                }
            }

            .copied-button {
                padding: 13px 28.5px;
                background-color: #196CDA;
                cursor: default;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 29px;
            top: 29px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover svg path {
                fill: #2683FF;
            }
        }

        .blur-content {
            position: absolute;
            top: 100%;
            left: 0;
            background-color: #F5F6FA;
            width: 100%;
            height: 70vh;
            z-index: 100;
            opacity: 0.3;
        }
    }
</style>
