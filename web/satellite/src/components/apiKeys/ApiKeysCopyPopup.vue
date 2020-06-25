// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="save-api-popup" v-if="isPopupShown">
        <h2 class="save-api-popup__title">Save Your Secret API Key! It Will Appear Only Once.</h2>
        <div class="save-api-popup__copy-area">
            <div class="save-api-popup__copy-area__key-area">
                <p class="save-api-popup__copy-area__key-area__key">{{ apiKeySecret }}</p>
            </div>
            <p class="save-api-popup__copy-area__copy-button" @click="onCopyClick">Copy</p>
        </div>
        <div class="save-api-popup__next-step-area">
            <span class="save-api-popup__next-step-area__label">Next Step:</span>
            <a
                class="save-api-popup__next-step-area__link"
                href="https://documentation.tardigrade.io/getting-started/uploading-your-first-object/set-up-uplink-cli"
                target="_blank"
                @click.self.stop="segmentTrack"
            >
                Set Up Uplink CLI
            </a>
            <VButton
                label="Done"
                width="156px"
                height="40px"
                :on-press="onCloseClick"
            />
        </div>
        <div class="blur-content"></div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import VButton from '@/components/common/VButton.vue';

import { SegmentEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
        HeaderlessInput,
    },
})
export default class ApiKeysCopyPopup extends Vue {
    /**
     * Indicates if component should be rendered.
     */
    @Prop({default: false})
    private readonly isPopupShown: boolean;
    @Prop({default: ''})
    private readonly apiKeySecret: string;

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$emit('closePopup');
    }

    /**
     * Copies api key secret to buffer.
     */
    public onCopyClick(): void {
        this.$copyText(this.apiKeySecret);
        this.$notify.success('Key saved to clipboard');
    }

    /**
     * Tracks if user checked uplink CLI docs.
     */
    public segmentTrack(): void {
        this.$segment.track(SegmentEvent.CLI_DOCS_VIEWED, {
            email: this.$store.getters.user.email,
        });
    }
}
</script>

<style scoped lang="scss">
    .save-api-popup {
        padding: 32px 40px;
        background-color: #fff;
        border-radius: 24px;
        margin-top: 29px;
        max-width: 94.8%;
        height: auto;
        position: relative;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 29px;
            margin-bottom: 26px;
        }

        &__copy-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            background-color: #f5f6fa;
            padding: 29px 32px 29px 24px;
            border-radius: 12px;
            position: relative;
            margin-bottom: 20px;

            &__key-area {

                &__key {
                    margin: 0;
                    font-size: 16px;
                    line-height: 21px;
                    word-break: break-all;
                }
            }

            &__copy-button {
                padding: 11px 22px;
                margin: 0 0 0 20px;
                background: #fff;
                border-radius: 6px;
                font-size: 15px;
                cursor: pointer;
                color: #2683ff;

                &:hover {
                    color: #fff;
                    background-color: #2683ff;
                }
            }
        }

        &__next-step-area {
            display: flex;
            justify-content: flex-end;
            align-items: center;
            width: 100%;

            &__label {
                font-size: 15px;
                line-height: 49px;
                letter-spacing: -0.100741px;
                color: #a0a0a0;
                margin-right: 15px;
            }

            &__link {
                display: flex;
                align-items: center;
                justify-content: center;
                border: 1px solid #2683ff;
                border-radius: 6px;
                width: 154px;
                height: 38px;
                font-size: 15px;
                line-height: 22px;
                color: #2683ff;
                margin-right: 15px;

                &:hover {
                    color: #fff;
                    background-color: #2683ff;
                }
            }
        }

        .blur-content {
            position: absolute;
            top: 100%;
            left: 0;
            background-color: #f5f6fa;
            width: 100%;
            height: 70.5vh;
            z-index: 100;
            opacity: 0.3;
        }
    }
</style>
