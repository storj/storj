// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
    <div>
        <div class="access-grant__modal-container__header-container">
            <AccessGrantsIcon v-if="accessSelected" />
            <S3Icon v-if="s3Selected" />
            <CLIIcon v-if="apiKeySelected" />
            <div class="access-grant__modal-container__header-container__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon />
            </div>
            <h2 class="access-grant__modal-container__header-container__title-complete">{{ accessName }}&nbsp;Created</h2>
        </div>
        <div class="access-grant__modal-container__body-container__created">
            <p v-if="!s3AndAccessSelected">Now copy and save the {{ checkedText[checkedType][0] }} will only appear once. Click on the {{ checkedText[checkedType][1] }}</p>
            <p v-else>Now copy and save the Access Grant and S3 Credentials as they will only appear once.</p>
        </div>
        <div v-if="accessSelected">
            <div class="access-grant__modal-container__generated-credentials__label first">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    Access Grant
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/access/access-grants"
                    target="_blank"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ access }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(access)"
                >
            </div>
        </div>
        <div v-if="s3Selected">
            <div class="access-grant__modal-container__generated-credentials__label first">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    Access Key
                </span>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ gatewayCredentials.accessKeyId }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(gatewayCredentials.accessKeyId)"
                >
            </div>
            <div class="access-grant__modal-container__generated-credentials__label">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    Secret Key
                </span>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ gatewayCredentials.secretKey }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(gatewayCredentials.secretKey)"
                >
            </div>
            <div class="access-grant__modal-container__generated-credentials__label">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    Endpoint
                </span>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ gatewayCredentials.endpoint }}
                </span>
                <img
                    class="clickable-image"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    target="_blank"
                    href="https://docs.storj.io/dcs/concepts/satellite"
                    @click="onCopyClick(gatewayCredentials.endpoint)"
                >
            </div>
        </div>
        <div v-if="apiKeySelected">
            <div class="access-grant__modal-container__generated-credentials__label first">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    Satellite Address
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/satellite"
                    target="_blank"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ satelliteAddress }}
                </span>
                <img
                    class="clickable-image"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    alt="copy icon"
                    @click="onCopyClick(satelliteAddress)"
                >
            </div>
            <div class="access-grant__modal-container__generated-credentials__label">
                <span class="access-grant__modal-container__generated-credentials__label__text">
                    API Key
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key"
                    target="_blank"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div
                class="access-grant__modal-container__generated-credentials"
            >
                <span class="access-grant__modal-container__generated-credentials__text">
                    {{ restrictedKey }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(restrictedKey)"
                >
            </div>
        </div>
        <div v-if="s3AndAccessSelected" class="multiple-section">
            <div class="multiple-section__access">
                <div class="access-grant__modal-container__header-container">
                    <AccessGrantsIcon />
                </div>
                <div>
                    <div class="access-grant__modal-container__generated-credentials__label first">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Access Grant
                        </span>
                        <a
                            href="https://docs.storj.io/dcs/concepts/access/access-grants/"
                            target="_blank"
                        >
                            <img
                                class="tooltip-icon"
                                alt="tooltip icon"
                                src="/static/static/images/accessGrants/create-access_information.png"
                            >
                        </a>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ access }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="/static/static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(access)"
                        >
                    </div>
                </div>
            </div>

            <div class="multiple-section__s3">
                <S3Icon />
                <div>
                    <div class="access-grant__modal-container__generated-credentials__label first">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Access Key
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.accessKeyId }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="/static/static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(gatewayCredentials.accessKeyId)"
                        >
                    </div>
                    <div class="access-grant__modal-container__generated-credentials__label">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Secret Key
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.secretKey }}
                        </span>
                        <img
                            class="clickable-image"
                            alt="copy icon"
                            src="/static/static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(gatewayCredentials.secretKey)"
                        >
                    </div>
                    <div class="access-grant__modal-container__generated-credentials__label">
                        <span class="access-grant__modal-container__generated-credentials__label__text">
                            Endpoint
                        </span>
                    </div>
                    <div
                        class="access-grant__modal-container__generated-credentials"
                    >
                        <span class="access-grant__modal-container__generated-credentials__text">
                            {{ gatewayCredentials.endpoint }}
                        </span>
                        <img
                            class="clickable-image"
                            src="/static/static/images/accessGrants/create-access_copy-icon.png"
                            @click="onCopyClick(gatewayCredentials.endpoint)"
                        >
                    </div>
                </div>
            </div>
        </div>
        <div v-if="s3Included" class="access-grant__modal-container__credential-buttons__container-s3">
            <a
                href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                target="_blank"
                rel="noopener noreferrer"
                @click="trackPageVisit('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway')"
            >
                <v-button
                    label="Learn More"
                    width="150px"
                    height="50px"
                    is-transparent="true"
                    font-size="16px"
                    class="access-grant__modal-container__footer-container__learn-more-button"
                />
            </a>
            <v-button
                label="Download .txt"
                font-size="16px"
                width="182px"
                height="50px"
                class="access-grant__modal-container__credential-buttons__download-button"
                :is-green-white="areCredentialsDownloaded"
                :on-press="downloadCredentials"
            />
        </div>
        <div v-else class="access-grant__modal-container__credential-buttons__container">
            <v-button
                label="Download .txt"
                font-size="16px"
                width="182px"
                height="50px"
                class="access-grant__modal-container__credential-buttons__download-button"
                :is-green-white="areCredentialsDownloaded"
                :on-press="downloadCredentials"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';

import { MetaUtils } from '@/utils/meta';
import { Download } from '@/utils/download';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';

import AccessGrantsIcon from '@/../static/images/accessGrants/accessGrantsIcon.svg';
import CLIIcon from '@/../static/images/accessGrants/cli.svg';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import S3Icon from '@/../static/images/accessGrants/s3.svg';

// @vue/component
@Component({
    components: {
        AccessGrantsIcon,
        CLIIcon,
        CloseCrossIcon,
        S3Icon,
        VButton,
    },
})

export default class GrantCreatedModal extends Vue {
    @Prop({ default: 'Default' })
    private readonly checkedType: string;
    @Prop({ default: 'Default' })
    private readonly restrictedKey: string;
    @Prop({ default: 'Default' })
    private readonly accessName: string;
    @Prop({ default: 'Default' })
    private readonly access: string;

    private areCredentialsDownloaded = false;
    private isAccessGrantCopied = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Global isLoading Variable
     **/
    private isLoading = false;
    private checkedText: Record<string, string[]> = { access: ['Access Grant as it','information icon to learn more.'], s3: ['S3 credentials as they','Learn More button to access the documentation.'],api: ['Satellite Address and API Key as they','information icons to learn more.'] };
    public currentDate = new Date().toISOString();
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    public onCloseClick(): void {
        this.$emit('close-modal');
    }

    public onCopyClick(item): void {
        this.$copyText(item);
        this.$notify.success(`credential was copied successfully`);
    }

    /**
     * Returns generated gateway credentials from store.
     */
    public get gatewayCredentials(): EdgeCredentials {
        return this.$store.state.accessGrantsModule.gatewayCredentials;
    }

    /**
     * Whether api is selected
     * */
    public get apiKeySelected(): boolean {
        return this.checkedType === 'api';
    }

    /**
     * Whether access is selected
     * */
    public get accessSelected(): boolean {
        return this.checkedType === 'access';
    }

    /**
     * Whether s3 is selected
     * */
    public get s3Selected(): boolean {
        return this.checkedType === 's3';
    }

    /**
     * Whether s3 access is what is/part of selected types
     **/
    public get s3Included(): boolean {
        return this.checkedType.includes('s3');
    }

    /**
     * Whether multiple access types are being created
    * */
    public get s3AndAccessSelected(): boolean {
        return this.s3Included && this.checkedType.includes('access');
    }

    public onCopyAccessGrantClick(): void {
        this.$copyText(this.restrictedKey);
        this.isAccessGrantCopied = true;
        this.analytics.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
        this.$notify.success(`Access Grant was copied successfully`);
    }

    /**
     * Downloads credentials to .txt file
     */
    public downloadCredentials(): void {
        let type = this.checkedType;
        if (this.s3AndAccessSelected)
            type = 's3Access';
        const credentialMap = {
            access: [`access grant: ${this.access}`],
            s3: [`access key: ${this.gatewayCredentials.accessKeyId}\nsecret key: ${this.gatewayCredentials.secretKey}\nendpoint: ${this.gatewayCredentials.endpoint}`],
            api: [`satellite address: ${this.satelliteAddress}\nrestricted key: ${this.restrictedKey}`],
            s3Access: [`access grant: ${this.access}\naccess key: ${this.gatewayCredentials.accessKeyId}\nsecret key: ${this.gatewayCredentials.secretKey}\nendpoint: ${this.gatewayCredentials.endpoint}`],
        };
        this.areCredentialsDownloaded = true;
        Download.file(credentialMap[type], `${this.checkedType}-credentials-${this.currentDate}.txt`);
        this.analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
    }

    /**
     * Opens S3 documentation in a new tab
     */
    public learnMore(): void{
        window.open('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway', '_blank');
    }

    /**
     * Sends "trackPageVisit" event to segment and opens link.
     */
    public trackPageVisit(link: string): void {
        this.analytics.pageVisit(link);
    }
}
</script>

<style scoped lang="scss">
    .multiple-section {

        &__close-icon {
            position: absolute;
            top: 34px;
            right: 34px;
        }

        &__access {
            margin-top: 20px;
        }

        &__s3 {
            border-top: 1px solid #e5e7eb;
            margin-top: 20px;
            padding-top: 20px;
        }
    }

    .button-icon {
        margin-right: 5px;
    }

    .clickable-image {
        cursor: pointer;
    }

    .tooltip-icon {
        display: flex;
        width: 14px;
        height: 14px;
        cursor: pointer;
    }

    @mixin generated-text {
        margin-top: 20px;
        align-items: center;
        padding: 10px 16px;
        background: #ebeef1;
        border: 1px solid #c8d3de;
        border-radius: 7px;
    }

    .access-grant {
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: flex-start;
        justify-content: center;

        & > * {
            font-family: sans-serif;
        }

        &__modal-container {
            background: #fff;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            position: relative;
            padding: 25px 40px;
            margin-top: 40px;
            width: 410px;
            height: auto;

            &__generated-credentials {
                @include generated-text;

                margin: 0 0 4px;
                display: flex;
                justify-content: space-between;

                &__text {
                    width: 90%;
                    text-overflow: ellipsis;
                    overflow-x: hidden;
                    white-space: nowrap;
                }

                &__label {
                    display: flex;
                    margin: 24px 0 8px;
                    align-items: center;

                    &.first {
                        margin-top: 8px;
                    }

                    &__text {
                        font-family: sans-serif;
                        font-size: 14px;
                        font-weight: 700;
                        line-height: 20px;
                        letter-spacing: 0;
                        text-align: left;
                        padding: 0 6px 0 0;
                    }
                }
            }

            &__credential-buttons {

                &__container-s3 {
                    display: flex;
                    justify-content: space-between;
                    margin: 15px 0;
                }

                &__container {
                    display: flex;
                    justify-content: center;
                    margin: 15px 0;
                }
            }

            &__header-container {
                text-align: left;
                display: grid;
                grid-template-columns: 2fr 1fr;
                width: 100%;
                padding-top: 10px;

                &__title {
                    grid-column: 1;
                }

                &__title-complete {
                    grid-column: 1;
                    margin-top: 10px;
                }

                &__close-cross-container {
                    grid-column: 2;
                    margin: auto 0 auto auto;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    right: 30px;
                    top: 30px;
                    height: 24px;
                    width: 24px;
                    cursor: pointer;
                }

                &__close-cross-container:hover .close-cross-svg-path {
                    fill: #2683ff;
                }
            }

            &__body-container {
                display: grid;
                grid-template-columns: 1fr 6fr;
                grid-template-rows: auto auto auto auto auto auto;
                grid-row-gap: 24px;
                width: 100%;
                padding-top: 10px;
                margin-top: 24px;

                &__created {
                    width: 100%;
                    text-align: left;
                    display: grid;
                    font-family: 'font_regular', sans-serif;
                    font-size: 16px;
                    margin-top: 15px;
                    row-gap: 4ch;
                    padding-top: 10px;

                    p {
                        font-style: normal;
                        font-weight: 400;
                        font-size: 14px;
                        line-height: 20px;
                        overflow-wrap: break-word;
                        text-align: left;
                    }
                }
            }

            &__footer-container {
                display: flex;
                width: 100%;
                justify-content: flex-start;
                margin-top: 16px;

                & :deep(.container:first-of-type) {
                    margin-right: 8px;
                }

                &__learn-more-button {
                    padding: 0 15px;
                }

                &__copy-button {
                    width: 49% !important;
                    margin-right: 10px;
                }

                &__download-button {
                    width: 49% !important;
                }
            }
        }
    }

    @media screen and (max-width: 500px) {

        .access-grant__modal-container {
            width: auto;
            max-width: 80vw;
            padding: 30px 24px;

            &__body-container {
                grid-template-columns: 1.2fr 6fr;
            }
        }
    }
</style>
