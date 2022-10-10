// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="grants">
        <AccessGrantsIcon v-if="accessSelected" />
        <S3Icon v-if="s3Selected" />
        <CLIIcon v-if="apiKeySelected" />
        <h2 class="grants__title">{{ accessName }}&nbsp;Created</h2>
        <p v-if="!s3AndAccessSelected" class="grants__created">Now copy and save the {{ checkedText[checkedType][0] }} will only appear once. Click on the {{ checkedText[checkedType][1] }}</p>
        <p v-else class="grants__created">Now copy and save the Access Grant and S3 Credentials as they will only appear once.</p>
        <template v-if="accessSelected">
            <div class="grants__label first">
                <span class="grants__label__text">
                    Access Grant
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/access/access-grants"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div
                class="grants__generated-credentials"
            >
                <span class="grants__generated-credentials__text">
                    {{ access }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(access)"
                >
            </div>
        </template>
        <template v-if="s3Selected">
            <div class="grants__label first">
                <span class="grants__label__text">
                    Access Key
                </span>
            </div>
            <div class="grants__generated-credentials">
                <span class="grants__generated-credentials__text">
                    {{ gatewayCredentials.accessKeyId }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(gatewayCredentials.accessKeyId)"
                >
            </div>
            <div class="grants__label">
                <span class="grants__label__text">
                    Secret Key
                </span>
            </div>
            <div class="grants__generated-credentials">
                <span class="grants__generated-credentials__text">
                    {{ gatewayCredentials.secretKey }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(gatewayCredentials.secretKey)"
                >
            </div>
            <div class="grants__label">
                <span class="grants__label__text">
                    Endpoint
                </span>
            </div>
            <div class="grants__generated-credentials">
                <span class="grants__generated-credentials__text">
                    {{ gatewayCredentials.endpoint }}
                </span>
                <img
                    class="clickable-image"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    alt="copy"
                    @click="onCopyClick(gatewayCredentials.endpoint)"
                >
            </div>
        </template>
        <template v-if="apiKeySelected">
            <div class="grants__label first">
                <span class="grants__label__text">
                    Satellite Address
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/satellite"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div class="grants__generated-credentials">
                <span class="grants__generated-credentials__text">
                    {{ satelliteAddress }}
                </span>
                <img
                    class="clickable-image"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    alt="copy icon"
                    @click="onCopyClick(satelliteAddress)"
                >
            </div>
            <div class="grants__label">
                <span class="grants__label__text">
                    API Key
                </span>
                <a
                    href="https://docs.storj.io/dcs/concepts/access/access-grants/api-key"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    <img
                        class="tooltip-icon"
                        alt="tooltip icon"
                        src="/static/static/images/accessGrants/create-access_information.png"
                    >
                </a>
            </div>
            <div class="grants__generated-credentials">
                <span class="grants__generated-credentials__text">
                    {{ restrictedKey }}
                </span>
                <img
                    class="clickable-image"
                    alt="copy icon"
                    src="/static/static/images/accessGrants/create-access_copy-icon.png"
                    @click="onCopyClick(restrictedKey)"
                >
            </div>
        </template>
        <template v-if="s3AndAccessSelected" class="multiple-section">
            <div class="multiple-section__access">
                <AccessGrantsIcon />
                <div class="grants__label first">
                    <span class="grants__label__text">
                        Access Grant
                    </span>
                    <a
                        href="https://docs.storj.io/dcs/concepts/access/access-grants/"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        <img
                            class="tooltip-icon"
                            alt="tooltip icon"
                            src="/static/static/images/accessGrants/create-access_information.png"
                        >
                    </a>
                </div>
                <div class="grants__generated-credentials">
                    <span class="grants__generated-credentials__text">
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

            <div class="multiple-section__s3">
                <S3Icon />
                <div class="grants__label first">
                    <span class="grants__label__text">
                        Access Key
                    </span>
                </div>
                <div class="grants__generated-credentials">
                    <span class="grants__generated-credentials__text">
                        {{ gatewayCredentials.accessKeyId }}
                    </span>
                    <img
                        class="clickable-image"
                        alt="copy icon"
                        src="/static/static/images/accessGrants/create-access_copy-icon.png"
                        @click="onCopyClick(gatewayCredentials.accessKeyId)"
                    >
                </div>
                <div class="grants__label">
                    <span class="grants__label__text">
                        Secret Key
                    </span>
                </div>
                <div class="grants__generated-credentials">
                    <span class="grants__generated-credentials__text">
                        {{ gatewayCredentials.secretKey }}
                    </span>
                    <img
                        class="clickable-image"
                        alt="copy icon"
                        src="/static/static/images/accessGrants/create-access_copy-icon.png"
                        @click="onCopyClick(gatewayCredentials.secretKey)"
                    >
                </div>
                <div class="grants__label">
                    <span class="grants__label__text">
                        Endpoint
                    </span>
                </div>
                <div class="grants__generated-credentials">
                    <span class="grants__generated-credentials__text">
                        {{ gatewayCredentials.endpoint }}
                    </span>
                    <img
                        class="clickable-image"
                        src="/static/static/images/accessGrants/create-access_copy-icon.png"
                        alt="copy"
                        @click="onCopyClick(gatewayCredentials.endpoint)"
                    >
                </div>
            </div>
        </template>
        <div v-if="s3Included" class="grants__buttons">
            <a
                class="link"
                href="https://docs.storj.io/dcs/api-reference/s3-compatible-gateway"
                target="_blank"
                rel="noopener noreferrer"
                @click="trackPageVisit('https://docs.storj.io/dcs/api-reference/s3-compatible-gateway')"
            >
                <v-button
                    label="Learn More"
                    height="48px"
                    is-transparent="true"
                    font-size="14px"
                    class="grants__buttons__learn-more"
                />
            </a>
            <v-button
                label="Download .txt"
                font-size="14px"
                height="48px"
                class="grants__buttons__download-button"
                :is-green-white="areCredentialsDownloaded"
                :on-press="downloadCredentials"
            />
        </div>
        <div v-else class="grants__buttons">
            <v-button
                label="Download .txt"
                font-size="14px"
                width="182px"
                height="48px"
                class="grants__buttons__download-button"
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
import S3Icon from '@/../static/images/accessGrants/s3.svg';

// @vue/component
@Component({
    components: {
        AccessGrantsIcon,
        CLIIcon,
        S3Icon,
        VButton,
    },
})

export default class GrantCreated extends Vue {
    @Prop({ default: 'Default' })
    private readonly checkedType: string;
    @Prop({ default: 'Default' })
    private readonly restrictedKey: string;
    @Prop({ default: 'Default' })
    private readonly accessName: string;
    @Prop({ default: 'Default' })
    private readonly access: string;

    private areCredentialsDownloaded = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    private checkedText: Record<string, string[]> = { access: ['Access Grant as it','information icon to learn more.'], s3: ['S3 credentials as they','Learn More button to access the documentation.'],api: ['Satellite Address and API Key as they','information icons to learn more.'] };
    public currentDate = new Date().toISOString();
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    public onCopyClick(item): void {
        this.$copyText(item);
        this.$notify.success(`Credential was copied successfully.`);
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

    /**
     * Downloads credentials to .txt file
     */
    public downloadCredentials(): void {
        let type = this.checkedType;
        if (this.s3AndAccessSelected)
            type = 's3Access';
        const credentialMap = {
            access: [this.access],
            s3: [`access key: ${this.gatewayCredentials.accessKeyId}\nsecret key: ${this.gatewayCredentials.secretKey}\nendpoint: ${this.gatewayCredentials.endpoint}`],
            api: [`satellite address: ${this.satelliteAddress}\nrestricted key: ${this.restrictedKey}`],
            s3Access: [`access grant: ${this.access}\naccess key: ${this.gatewayCredentials.accessKeyId}\nsecret key: ${this.gatewayCredentials.secretKey}\nendpoint: ${this.gatewayCredentials.endpoint}`],
        };
        this.areCredentialsDownloaded = true;
        Download.file(credentialMap[type], `${this.checkedType}-credentials-${this.currentDate}.txt`);
        this.analytics.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
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
    .grants {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        font-family: 'font_regular', sans-serif;
        padding: 32px;
        max-width: 350px;

        @media screen and (max-width: 470px) {
            max-width: 300px;
            padding: 32px 16px;
        }

        @media screen and (max-width: 380px) {
            max-width: 250px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #14142b;
            text-align: left;
            margin-top: 10px;
            word-break: break-word;
        }

        &__label {
            display: flex;
            margin-top: 24px;
            align-items: center;

            &.first {
                margin-top: 8px;
            }

            &__text {
                font-size: 14px;
                font-weight: 700;
                line-height: 20px;
                letter-spacing: 0;
                text-align: left;
                padding: 0 6px 0 0;
            }
        }

        &__generated-credentials {
            margin-top: 10px;
            align-items: center;
            padding: 10px 16px;
            background: #ebeef1;
            border: 1px solid #c8d3de;
            border-radius: 7px;
            display: flex;
            justify-content: space-between;
            max-width: calc(100% - 32px);
            width: calc(100% - 32px);

            &__text {
                width: 90%;
                text-align: left;
                text-overflow: ellipsis;
                overflow-x: hidden;
                white-space: nowrap;
            }
        }

        &__buttons {
            display: flex;
            align-items: center;
            margin-top: 32px;
            width: 100%;
            column-gap: 8px;

            @media screen and (max-width: 470px) {
                flex-direction: column;
                column-gap: unset;
                row-gap: 8px;
            }

            &__learn-more,
            &__download-button {
                padding: 0 15px;

                @media screen and (max-width: 470px) {
                    width: calc(100% - 30px) !important;
                }
            }
        }

        &__created {
            font-size: 14px;
            line-height: 20px;
            overflow-wrap: break-word;
            text-align: left;
            margin-top: 32px;
        }
    }

    .multiple-section {

        &__access {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            width: 100%;
            margin-top: 20px;
        }

        &__s3 {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            width: 100%;
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

    .link {
        @media screen and (max-width: 470px) {
            width: 100%;
        }
    }
</style>
