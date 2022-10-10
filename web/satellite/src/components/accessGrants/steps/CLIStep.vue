// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="cli-container" :class="{ 'border-radius': isOnboardingTour }">
        <BackIcon class="cli-container__back-icon" @click="onBackClick" />
        <h1 class="cli-container__title">Create Access Grant in CLI</h1>
        <p class="cli-container__sub-title">
            Run the 'setup' command in the uplink CLI and input the satellite address and token below when prompted to generate your access grant.
        </p>
        <div class="cli-container__token-area">
            <p class="cli-container__token-area__label">Satellite Address</p>
            <div class="cli-container__token-area__container">
                <p ref="addressContainer" class="cli-container__token-area__container__token" @click="selectAddress">{{ satelliteAddress }}</p>
                <VButton
                    class="cli-container__token-area__container__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    :on-press="onCopyAddressClick"
                />
            </div>
            <p class="cli-container__token-area__label">API Key</p>
            <div class="cli-container__token-area__container">
                <p class="cli-container__token-area__container__token">{{ restrictedKey }}</p>
                <VButton
                    class="cli-container__token-area__container__button"
                    label="Copy"
                    width="66px"
                    height="30px"
                    :on-press="onCopyTokenClick"
                />
            </div>
        </div>
        <VButton
            class="cli-container__done-button"
            label="Done"
            width="100%"
            height="48px"
            :on-press="onDoneClick"
        />
        <a
            class="cli-container__docs-link"
            href="https://docs.storj.io/getting-started/generate-access-grants-and-tokens/generate-a-token"
            target="_blank"
            rel="noopener noreferrer"
        >
            Visit the Docs
        </a>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';

import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

// @vue/component
@Component({
    components: {
        BackIcon,
        VButton,
    },
})
export default class CLIStep extends Vue {
    public key = '';
    public restrictedKey = '';
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    public $refs!: {
        addressContainer: HTMLElement;
    };

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public mounted(): void {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).path);
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }

    /**
     * Holds on done button click logic.
     * Redirects to upload step.
     */
    public onDoneClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.path);
        this.isOnboardingTour ? this.$router.push(RouteConfig.ProjectDashboard.path) : this.$router.push(RouteConfig.AccessGrants.path);
    }

    /**
     * Holds selecting address logic for click event.
     */
    public selectAddress(): void {
        const range: Range = document.createRange();
        const selection: Selection | null = window.getSelection();

        range.selectNodeContents(this.$refs.addressContainer);

        if (selection) {
            selection.removeAllRanges();
            selection.addRange(range);
        }
    }

    /**
     * Holds on copy button click logic.
     * Copies satellite address to clipboard.
     */
    public onCopyAddressClick(): void {
        this.$copyText(this.satelliteAddress);
        this.$notify.success('Satellite address was copied successfully');
    }

    /**
     * Holds on copy button click logic.
     * Copies token to clipboard.
     */
    public onCopyTokenClick(): void {
        this.$copyText(this.restrictedKey);
        this.$notify.success('Token was copied successfully');
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
    .cli-container {
        height: calc(100% - 60px);
        max-width: 485px;
        padding: 30px 65px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 6px;

        &__back-icon {
            position: absolute;
            top: 40px;
            left: 40px;
            cursor: pointer;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 10px;
        }

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 50px;
        }

        &__token-area {
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            width: 100%;
            margin-bottom: 30px;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #354049;
                margin: 0 0 5px;
            }

            &__container {
                display: flex;
                align-items: center;
                padding: 10px 10px 10px 20px;
                width: calc(100% - 30px);
                border: 1px solid rgb(56 75 101 / 40%);
                border-radius: 6px;
                margin-bottom: 20px;

                &__token {
                    font-size: 16px;
                    line-height: 21px;
                    color: #384b65;
                    text-overflow: ellipsis;
                    white-space: nowrap;
                    overflow: hidden;
                    margin: 0;
                }

                &__button {
                    min-width: 66px;
                    min-height: 30px;
                    margin-left: 10px;
                }
            }
        }

        &__docs-link {
            font-family: 'font_medium', sans-serif;
            font-weight: 600;
            font-size: 16px;
            line-height: 23px;
            color: #0068dc;
            margin: 16px 0;
        }
    }

    .border-radius {
        border-radius: 6px;
    }
</style>
