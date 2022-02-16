// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="gateway" :class="{ 'border-radius': isOnboardingTour }">
        <BackIcon class="gateway__back-icon" @click="onBackClick" />
        <h1 class="gateway__title">S3 Gateway</h1>
        <div class="gateway__container">
            <h3 class="gateway__container__title">
                Generate S3 Gateway Credentials
            </h3>
            <p class="gateway__container__disclaimer">
                By generating gateway credentials, you are opting in to Server-side encryption
            </p>
            <VButton
                v-if="!areKeysVisible"
                class="gateway__container__button"
                label="Generate Credentials"
                width="calc(100% - 4px)"
                height="48px"
                :is-blue-white="true"
                :on-press="onGenerateCredentialsClick"
                :is-disabled="isLoading"
            />
            <div v-else class="gateway__container__keys-area">
                <div class="gateway__container__keys-area__label-area">
                    <h3 class="gateway__container__keys-area__label-area__label">Access Key</h3>
                    <VInfo class="gateway__container__keys-area__label-area__info-button">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="gateway__container__keys-area__label-area__info-button__message">
                                The access key ID uniquely identifies your account.
                            </p>
                        </template>
                    </VInfo>
                </div>
                <div class="gateway__container__keys-area__key">
                    <p class="gateway__container__keys-area__key__value">{{ gatewayCredentials.accessKeyId }}</p>
                    <VButton
                        class="gateway__container__keys-area__key__button"
                        label="Copy"
                        width="66px"
                        height="30px"
                        :on-press="onCopyAccessClick"
                    />
                </div>
                <div class="gateway__container__keys-area__label-area">
                    <h3 class="gateway__container__keys-area__label-area__label">Secret Key</h3>
                    <VInfo class="gateway__container__keys-area__label-area__info-button">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="gateway__container__keys-area__label-area__info-button__message">
                                Secret access keys are—as the name implies—secrets, like your password.
                            </p>
                        </template>
                    </VInfo>
                </div>
                <div class="gateway__container__keys-area__key">
                    <p class="gateway__container__keys-area__key__value">{{ gatewayCredentials.secretKey }}</p>
                    <VButton
                        class="gateway__container__keys-area__key__button"
                        label="Copy"
                        width="66px"
                        height="30px"
                        :on-press="onCopySecretClick"
                    />
                </div>
                <div class="gateway__container__keys-area__label-area">
                    <h3 class="gateway__container__keys-area__label-area__label">End Point</h3>
                    <VInfo class="gateway__container__keys-area__label-area__info-button">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="gateway__container__keys-area__label-area__info-button__message">
                                The service to which you want to establish the connection.
                            </p>
                        </template>
                    </VInfo>
                </div>
                <div class="gateway__container__keys-area__key">
                    <p class="gateway__container__keys-area__key__value">{{ gatewayCredentials.endpoint }}</p>
                    <VButton
                        class="gateway__container__keys-area__key__button"
                        label="Copy"
                        width="66px"
                        height="30px"
                        :on-press="onCopyEndpointClick"
                    />
                </div>
            </div>
        </div>
        <VButton
            label="Done"
            width="100%"
            height="48px"
            :on-press="onDoneClick"
            :is-disabled="!gatewayCredentials.accessKeyId"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import InfoIcon from '@/../static/images/accessGrants/info.svg';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

// @vue/component
@Component({
    components: {
        VButton,
        VInfo,
        BackIcon,
        InfoIcon,
    },
})
export default class GatewayStep extends Vue {
    private key = '';
    private restrictedKey = '';
    private access = '';
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public areKeysVisible = false;
    public isLoading = false;

    /**
     * Lifecycle hook after initial render.
     * Sets local access from props value.
     */
    public mounted(): void {
        if (!this.$route.params.access && !this.$route.params.key && !this.$route.params.resctrictedKey) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.access = this.$route.params.access;
        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;
    }

    /**
     * Holds on copy access key button click logic.
     * Copies key to clipboard.
     */
    public onCopyAccessClick(): void {
        this.$copyText(this.gatewayCredentials.accessKeyId);
        this.$notify.success('Key was copied successfully');
    }

    /**
     * Holds on copy secret key button click logic.
     * Copies secret to clipboard.
     */
    public onCopySecretClick(): void {
        this.$copyText(this.gatewayCredentials.secretKey);
        this.$notify.success('Secret was copied successfully');
    }

    /**
     * Holds on copy endpoint button click logic.
     * Copies endpoint to clipboard.
     */
    public onCopyEndpointClick(): void {
        this.$copyText(this.gatewayCredentials.endpoint);
        this.$notify.success('Endpoint was copied successfully');
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).name,
            params: {
                access: this.access,
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Holds on done button click logic.
     * Proceed to upload data step.
     */
    public onDoneClick(): void {
        this.isOnboardingTour ? this.$router.push(RouteConfig.ProjectDashboard.path) : this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Holds on generate credentials button click logic.
     * Generates gateway credentials.
     */
    public async onGenerateCredentialsClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: this.access});

            await this.$notify.success('Gateway credentials were generated successfully');

            await this.analytics.eventTriggered(AnalyticsEvent.GATEWAY_CREDENTIALS_CREATED);

            this.areKeysVisible = true;
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.isLoading = false;
    }

    /**
     * Returns generated gateway credentials from store.
     */
    public get gatewayCredentials(): EdgeCredentials {
        return this.$store.state.accessGrantsModule.gatewayCredentials;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }
}
</script>

<style scoped lang="scss">
    .gateway {
        height: calc(100% - 60px);
        padding: 30px 65px;
        max-width: 475px;
        min-width: 475px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0;
        }

        &__container {
            background: #f5f6fa;
            border-radius: 6px;
            padding: 50px;
            margin: 55px 0 40px 0;
            width: calc(100% - 100px);

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 22px;
                line-height: 27px;
                color: #000;
                margin: 0 0 25px 0;
                text-align: center;
            }

            &__disclaimer {
                font-size: 16px;
                line-height: 28px;
                color: #000;
                margin: 0 0 25px 0;
                text-align: center;
            }

            &__keys-area {
                display: flex;
                flex-direction: column;
                align-items: flex-start;

                &__label-area {
                    display: flex;
                    align-items: center;
                    margin: 20px 0 10px 0;

                    &__label {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                        margin: 0;
                    }

                    &__info-button {
                        max-height: 20px;
                        cursor: pointer;
                        margin-left: 10px;

                        &:hover {

                            .ag-info-rect {
                                fill: #fff;
                            }

                            .ag-info-path {
                                fill: #2683ff;
                            }
                        }

                        &__message {
                            color: #586c86;
                            font-family: 'font_medium', sans-serif;
                            font-size: 16px;
                            line-height: 18px;
                        }
                    }
                }

                &__key {
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    border-radius: 9px;
                    padding: 10px;
                    width: calc(100% - 20px);
                    max-width: calc(100% - 20px);
                    border: 1px solid rgba(56, 75, 101, 0.4);
                    background-color: #fff;

                    &__value {
                        text-overflow: ellipsis;
                        overflow: hidden;
                        white-space: nowrap;
                        margin: 0;
                    }

                    &__button {
                        min-width: 66px;
                        min-height: 30px;
                        margin-left: 10px;
                    }
                }
            }
        }
    }

    .border-radius {
        border-radius: 6px;
    }

    ::v-deep .info__box__message {
        min-width: 300px;
    }
</style>
