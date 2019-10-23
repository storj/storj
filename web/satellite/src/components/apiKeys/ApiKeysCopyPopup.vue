// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="save-api-popup" v-if="isPopupShown">
        <h2 class="save-api-popup__title">Save Your API Key! It Will Appear Only Once.</h2>
        <div class="save-api-popup__copy-area">
            <div class="save-api-popup__copy-area__key-area">
                <p class="save-api-popup__copy-area__key-area__key">{{apiKeySecret}}</p>
            </div>
            <div class="copy-button" v-clipboard="apiKeySecret" @click="onCopyClick" v-if="!isCopiedButtonShown">
                <CopyButtonLabelIcon/>
                <p class="copy-button__label">Copy</p>
            </div>
            <div class="copied-button" v-if="isCopiedButtonShown">
                <CopyButtonLabelIcon/>
                <p class="copied-button__label">Copied</p>
            </div>
        </div>
        <div class="save-api-popup__close-cross-container" @click="onCloseClick">
            <CloseCrossIcon/>
        </div>
        <div class="blur-content"></div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import CopyButtonLabelIcon from '@/../static/images/apiKeys/copyButtonLabel.svg';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        HeaderlessInput,
        CopyButtonLabelIcon,
        CloseCrossIcon,
    },
})
export default class ApiKeysCopyPopup extends Vue {
    @Prop({default: false})
    private readonly isPopupShown: boolean;
    @Prop({default: ''})
    private readonly apiKeySecret: string;

    public isCopiedButtonShown: boolean = false;

    public onCloseClick(): void {
        this.isCopiedButtonShown = false;
        this.$emit('closePopup');
    }

    public async onCopyClick(): Promise<void> {
        await this.$notify.success('Key saved to clipboard');
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
        font-family: 'font_regular';

        &__title {
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

                &__key {
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

                &__label {
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

            &:hover .close-cross-svg-path {
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
