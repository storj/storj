// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generate-grant" :class="{ 'border-radius': isOnboardingTour }">
        <BackIcon class="generate-grant__back-icon" @click="onBackClick"/>
        <h1 class="generate-grant__title">Generate Access Grant</h1>
        <div class="generate-grant__warning">
            <div class="generate-grant__warning__header">
                <WarningIcon/>
                <h2 class="generate-grant__warning__header__label">This Information is Only Displayed Once</h2>
            </div>
            <p class="generate-grant__warning__message">
                Save this information in a password manager, or wherever you prefer to store sensitive information.
            </p>
        </div>
        <div class="generate-grant__grant-area">
            <h3 class="generate-grant__grant-area__label">Access Grant</h3>
            <div class="generate-grant__grant-area__container">
                <p class="generate-grant__grant-area__container__value">{{ access }}</p>
                <VButton
                    class="generate-grant__grant-area__container__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    :on-press="onCopyGrantClick"
                />
            </div>
        </div>
        <div class="generate-grant__gateway-area" v-if="isGatewayDropdownVisible">
            <div class="generate-grant__gateway-area__toggle" @click="toggleCredentialsVisibility">
                <h3 class="generate-grant__gateway-area__toggle__label">Gateway Credentials</h3>
                <ExpandIcon v-if="!areGatewayCredentialsVisible"/>
                <HideIcon v-else/>
            </div>
            <div class="generate-grant__gateway-area__container" v-if="areGatewayCredentialsVisible">
                <div class="generate-grant__gateway-area__container__beta">
                    <p class="generate-grant__gateway-area__container__beta__message">Gateway MT is currently in Beta</p>
                    <a
                        class="generate-grant__gateway-area__container__beta__link"
                        href="https://forum.storj.io/t/gateway-mt-beta-looking-for-testers/11324"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Learn More >
                    </a>
                </div>
                <div class="generate-grant__gateway-area__container__warning" v-if="!areKeysVisible">
                    <h3 class="generate-grant__gateway-area__container__warning__title">
                        Using Gateway Credentials Enables Server-Side Encryption.
                    </h3>
                    <p class="generate-grant__gateway-area__container__warning__disclaimer">
                        By generating gateway credentials, you are opting in to Server-side encryption
                    </p>
                    <VButton
                        class="generate-grant__gateway-area__container__warning__button"
                        label="Generate Gateway Credentials"
                        width="calc(100% - 4px)"
                        height="48px"
                        :is-blue-white="true"
                        :on-press="onGenerateCredentialsClick"
                        :is-disabled="isLoading"
                    />
                </div>
                <div class="generate-grant__gateway-area__container__keys-area" v-else>
                    <h3 class="generate-grant__gateway-area__container__keys-area__label">Access Key</h3>
                    <div class="generate-grant__gateway-area__container__keys-area__key">
                        <p class="generate-grant__gateway-area__container__keys-area__key__value">{{ gatewayCredentials.accessKeyId }}</p>
                        <VButton
                            class="generate-grant__gateway-area__container__keys-area__key__button"
                            label="Copy"
                            width="66px"
                            height="30px"
                            :on-press="onCopyAccessClick"
                        />
                    </div>
                    <h3 class="generate-grant__gateway-area__container__keys-area__label">Secret Key</h3>
                    <div class="generate-grant__gateway-area__container__keys-area__key">
                        <p class="generate-grant__gateway-area__container__keys-area__key__value">{{ gatewayCredentials.secretKey }}</p>
                        <VButton
                            class="generate-grant__gateway-area__container__keys-area__key__button"
                            label="Copy"
                            width="66px"
                            height="30px"
                            :on-press="onCopySecretClick"
                        />
                    </div>
                    <h3 class="generate-grant__gateway-area__container__keys-area__label">End Point</h3>
                    <div class="generate-grant__gateway-area__container__keys-area__key">
                        <p class="generate-grant__gateway-area__container__keys-area__key__value">{{ gatewayCredentials.endpoint }}</p>
                        <VButton
                            class="generate-grant__gateway-area__container__keys-area__key__button"
                            label="Copy"
                            width="66px"
                            height="30px"
                            :on-press="onCopyEndpointClick"
                        />
                    </div>
                </div>
            </div>
        </div>
        <VButton
            class="generate-grant__done-button"
            :class="{ 'extra-margin-top': !(isOnboardingTour || areGatewayCredentialsVisible) }"
            label="Done"
            width="100%"
            height="48px"
            :on-press="onDoneClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';
import WarningIcon from '@/../static/images/accessGrants/warning.svg';
import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';
import HideIcon from '@/../static/images/common/BlackArrowHide.svg';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { GatewayCredentials } from '@/types/accessGrants';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        BackIcon,
        WarningIcon,
        VButton,
        ExpandIcon,
        HideIcon,
    },
})
export default class ResultStep extends Vue {
    private key: string = '';
    private restrictedKey: string = '';

    public access: string = '';
    public isGatewayDropdownVisible: boolean = false;
    public areGatewayCredentialsVisible: boolean = false;
    public areKeysVisible: boolean = false;
    public isLoading: boolean = false;

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

        const requestURL = MetaUtils.getMetaContent('gateway-credentials-request-url');
        if (requestURL) this.isGatewayDropdownVisible = true;
    }

    /**
     * Toggles gateway credentials section visibility.
     */
    public toggleCredentialsVisibility(): void {
        this.areGatewayCredentialsVisible = !this.areGatewayCredentialsVisible;
    }

    /**
     * Holds on copy access grant button click logic.
     * Copies token to clipboard.
     */
    public onCopyGrantClick(): void {
        this.$copyText(this.access);
        this.$notify.success('Token was copied successfully');
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
        if (this.isOnboardingTour) {
            this.$router.push({
                name: RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant.with(RouteConfig.AccessGrantPassphrase)).name,
                params: {
                    key: this.key,
                    restrictedKey: this.restrictedKey,
                },
            });

            return;
        }

        if (this.accessGrantsAmount > 1) {
            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).name,
                params: {
                    key: this.key,
                    restrictedKey: this.restrictedKey,
                },
            });

            return;
        }

        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).name,
            params: {
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
        this.isLoading = true;

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, this.access);

            await this.$notify.success('Gateway credentials were generated successfully');
            this.areKeysVisible = true;

            const satelliteName: string = MetaUtils.getMetaContent('satellite-name');

            this.$segment.track(SegmentEvent.GENERATE_GATEWAY_CREDENTIALS_CLICKED, {
                satelliteName: satelliteName,
                email: this.$store.getters.user.email,
            });

            this.isLoading = false;
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Returns generated gateway credentials from store.
     */
    public get gatewayCredentials(): GatewayCredentials {
        return this.$store.state.accessGrantsModule.gatewayCredentials;
    }

    /**
     * Returns amount of access grants from store.
     */
    private get accessGrantsAmount(): number {
        return this.$store.state.accessGrantsModule.page.accessGrants.length;
    }
}
</script>

<style scoped lang="scss">
    .generate-grant {
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
            margin: 0 0 30px 0;
        }

        &__warning {
            padding: 15px;
            width: calc(100% - 32px);
            background: #fff9f7;
            border: 1px solid #f84b00;
            border-radius: 8px;

            &__header {
                display: flex;
                align-items: center;

                &__label {
                    font-style: normal;
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 19px;
                    color: #1b2533;
                    margin: 0 0 0 15px;
                }
            }

            &__message {
                font-size: 16px;
                line-height: 22px;
                color: #1b2533;
                margin: 8px 0 0 0;
            }
        }

        &__grant-area {
            margin: 20px;
            width: 100%;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 0 0 10px 0;
            }

            &__container {
                display: flex;
                align-items: center;
                border-radius: 9px;
                padding: 10px;
                width: calc(100% - 22px);
                border: 1px solid rgba(56, 75, 101, 0.4);

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

        &__gateway-area {
            display: flex;
            flex-direction: column;
            align-items: center;
            width: 100%;

            &__toggle {
                display: flex;
                align-items: center;
                cursor: pointer;
                justify-content: center;

                &__label {
                    font-family: 'font_bold', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #49515c;
                    margin: 0 15px 0 0;
                }
            }

            &__container {
                width: 100%;

                &__beta {
                    background-color: #effff9;
                    border: 1px solid #1a9666;
                    border-radius: 6px;
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    padding: 12px 20px;
                    margin-top: 20px;

                    &__message {
                        font-weight: bold;
                        font-size: 14px;
                        line-height: 19px;
                        color: #000;
                        margin: 0;
                    }

                    &__link {
                        font-weight: bold;
                        font-size: 14px;
                        line-height: 19px;
                        color: #1a9666;
                    }
                }

                &__warning {
                    margin-top: 20px;
                    background: #f5f6fa;
                    border-radius: 6px;
                    padding: 40px 50px;
                    width: calc(100% - 100px);

                    &__title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 18px;
                        line-height: 24px;
                        color: #000;
                        margin: 0 0 20px 0;
                        text-align: center;
                    }

                    &__disclaimer {
                        font-size: 16px;
                        line-height: 28px;
                        color: #000;
                        margin: 0 0 25px 0;
                        text-align: center;
                    }
                }

                &__keys-area {
                    display: flex;
                    flex-direction: column;
                    align-items: flex-start;

                    &__label {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 21px;
                        color: #354049;
                        margin: 20px 0 10px 0;
                    }

                    &__key {
                        display: flex;
                        align-items: center;
                        justify-content: space-between;
                        border-radius: 9px;
                        padding: 10px;
                        width: calc(100% - 20px);
                        border: 1px solid rgba(56, 75, 101, 0.4);

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

        &__done-button {
            margin-top: 20px;
        }
    }

    .border-radius {
        border-radius: 6px;
    }

    .extra-margin-top {
        margin-top: 76px;
    }
</style>
