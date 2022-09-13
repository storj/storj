// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generate-grant" :class="{ 'border-radius': isOnboardingTour }">
        <BackIcon class="generate-grant__back-icon" @click="onBackClick" />
        <h1 class="generate-grant__title">Generate Access Grant</h1>
        <div class="generate-grant__warning">
            <div class="generate-grant__warning__header">
                <WarningIcon />
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
                <VButton
                    class="generate-grant__grant-area__container__button"
                    label="Download"
                    width="80px"
                    height="30px"
                    :on-press="onDownloadGrantClick"
                />
            </div>
        </div>
        <VButton
            class="generate-grant__done-button"
            label="Done"
            width="100%"
            height="48px"
            :on-press="onDoneClick"
        />
        <p v-if="isGatewayLinkVisible" class="generate-grant__gateway-link" @click="navigateToGatewayStep">
            Generate S3 Gateway Credentials
        </p>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { Download } from '@/utils/download';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';

import WarningIcon from '@/../static/images/accessGrants/warning.svg';
import BackIcon from '@/../static/images/accessGrants/back.svg';

// @vue/component
@Component({
    components: {
        BackIcon,
        WarningIcon,
        VButton,
    },
})
export default class ResultStep extends Vue {
    private key = '';
    private restrictedKey = '';

    public access = '';
    public isGatewayLinkVisible = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Sets local access from props value.
     */
    public mounted(): void {
        if (!this.$route.params.access && !this.$route.params.key && !this.$route.params.resctrictedKey) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.access = this.$route.params.access;
        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;

        const requestURL = MetaUtils.getMetaContent('gateway-credentials-request-url');
        if (requestURL) this.isGatewayLinkVisible = true;
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
     * Holds on download access grant button click logic.
     * Downloads a file with the access called access-grant-<timestamp>.key
     */
    public onDownloadGrantClick(): void {
        const ts = new Date();
        const filename = 'access-grant-' + ts.toJSON() + '.key';

        Download.file(this.access, filename);

        this.$notify.success('Token was downloaded successfully');
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        if (this.accessGrantsAmount > 1) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).path);
            this.$router.push({
                name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.EnterPassphraseStep)).name,
                params: {
                    key: this.key,
                    restrictedKey: this.restrictedKey,
                },
            });

            return;
        }

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.CreatePassphraseStep)).path);
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
        this.analytics.pageVisit(RouteConfig.AccessGrants.path);
        this.isOnboardingTour ? this.$router.push(RouteConfig.ProjectDashboard.path) : this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Holds on link click logic.
     * Proceed to gateway step.
     */
    public navigateToGatewayStep(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.GatewayStep)).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.GatewayStep)).name,
            params: {
                access: this.access,
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
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
            margin: 0 0 30px;
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
                margin: 8px 0 0;
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
                margin: 0 0 10px;
            }

            &__container {
                display: flex;
                align-items: center;
                border-radius: 9px;
                padding: 10px;
                width: calc(100% - 22px);
                border: 1px solid rgb(56 75 101 / 40%);

                &__value {
                    text-overflow: ellipsis;
                    overflow: hidden;
                    white-space: nowrap;
                    margin: 0;
                }

                &__button {
                    min-width: 85px;
                    min-height: 30px;
                    margin-left: 10px;
                }
            }
        }

        &__done-button {
            margin-top: 20px;
        }

        &__gateway-link {
            font-weight: 600;
            font-size: 16px;
            line-height: 23px;
            text-align: center;
            color: #0068dc;
            margin: 30px 0 0;
            cursor: pointer;

            &:hover {
                text-decoration: underline;
            }
        }
    }

    .border-radius {
        border-radius: 6px;
    }
</style>
