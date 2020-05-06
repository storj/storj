// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="upload-data-area">
        <div class="upload-data-area__container">
            <h1 class="upload-data-area__container__title">Upload Data</h1>
            <p class="upload-data-area__container__sub-title">
                From here, you’ll set up Tardigrade to store data for your project using our S3 Gateway, Uplink CLI, or
                select from our growing library of connectors to build apps on Tardigrade.
            </p>
            <div class="upload-data-area__container__docs-area">
                <div class="upload-data-area__container__docs-area__option">
                    <h2 class="upload-data-area__container__docs-area__option__title">
                        Migrate Data from your Existing AWS buckets
                    </h2>
                    <img src="@/../static/images/onboardingTour/s3.png" alt="s3 gateway image">
                    <h3 class="upload-data-area__container__docs-area__option__sub-title">
                        S3 Gateway
                    </h3>
                    <p class="upload-data-area__container__docs-area__option__info">
                        Make the switch with Tardigrade’s S3 Gateway.
                    </p>
                    <a
                        class="upload-data-area__container__docs-area__option__link"
                        href="https://documentation.tardigrade.io/api-reference/s3-gateway"
                        target="_blank"
                    >
                        S3 Gateway Docs
                    </a>
                </div>
                <div class="upload-data-area__container__docs-area__option uplink-option">
                    <h2 class="upload-data-area__container__docs-area__option__title">
                        Upload Data from Your Local Environment
                    </h2>
                    <img src="@/../static/images/onboardingTour/uplinkcli.png" alt="uplink cli image">
                    <h3 class="upload-data-area__container__docs-area__option__sub-title">
                        Uplink CLI
                    </h3>
                    <p class="upload-data-area__container__docs-area__option__info">
                        Start uploading data from the command line.
                    </p>
                    <a
                        class="upload-data-area__container__docs-area__option__link"
                        href="https://documentation.tardigrade.io/api-reference/uplink-cli"
                        target="_blank"
                    >
                        Uplink CLI Docs
                    </a>
                </div>
                <div class="upload-data-area__container__docs-area__option">
                    <h2 class="upload-data-area__container__docs-area__option__title">
                        Use Tardigrade for your app’s storage layer
                    </h2>
                    <img src="@/../static/images/onboardingTour/connectors.png" alt="connectors image">
                    <h3 class="upload-data-area__container__docs-area__option__sub-title">
                        App Connectors
                    </h3>
                    <p class="upload-data-area__container__docs-area__option__info">
                        Integrate Tardigrade into your existing stack.
                    </p>
                    <a
                        class="upload-data-area__container__docs-area__option__link"
                        href="https://documentation.tardigrade.io/concepts/connectors"
                        target="_blank"
                    >
                        App Connectors
                    </a>
                </div>
            </div>
        </div>
        <VButton
            class="go-to-button"
            width="276px"
            height="40px"
            label="Go to Dashboard"
            :on-press="onButtonClick"
        />
        <span class="upload-data-area__support-info">
            Need help?
            <a class="upload-data-area__support-info__link" href="https://support.tardigrade.io/hc/en-us" target="_blank">
                Contact support
            </a>
        </span>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import { RouteConfig } from '@/router';

@Component({
    components: {
        VButton,
    },
})

export default class UploadDataStep extends Vue {
    /**
     * Holds button click logic.
     * Ends onboarding tour and redirects to project dashboard.
     */
    public onButtonClick(): void {
        this.$router.push(RouteConfig.ProjectDashboard.path);
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    h3,
    p {
        margin: 0;
    }

    .upload-data-area {
        display: flex;
        flex-direction: column;
        align-items: center;
        width: auto;
        font-family: 'font_regular', sans-serif;
        margin-top: 25px;

        &__container {
            padding: 60px;
            display: flex;
            flex-direction: column;
            align-items: center;
            background-color: rgba(255, 255, 255, 0.4);
            border-radius: 8px;
            margin-bottom: 50px;

            &__title {
                font-size: 32px;
                line-height: 39px;
                color: #1b2533;
                margin-bottom: 25px;
            }

            &__sub-title {
                font-size: 16px;
                line-height: 19px;
                color: #354049;
                margin-bottom: 35px;
                text-align: center;
                word-break: break-word;
                max-width: 800px;
            }

            &__docs-area {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__option {
                    padding: 30px;
                    display: flex;
                    flex-direction: column;
                    align-items: center;
                    border: 1px solid rgba(144, 155, 168, 0.4);
                    border-radius: 8px;

                    &__title {
                        font-size: 10px;
                        line-height: 15px;
                        text-align: center;
                        letter-spacing: 0.05em;
                        text-transform: uppercase;
                        color: #909ba8;
                        margin-bottom: 25px;
                        max-width: 181px;
                        word-break: break-word;
                    }

                    &__sub-title {
                        font-family: 'font_bold', sans-serif;
                        font-size: 16px;
                        line-height: 26px;
                        text-align: center;
                        color: #354049;
                        margin: 25px 0 5px 0;
                    }

                    &__info {
                        font-size: 14px;
                        line-height: 17px;
                        text-align: center;
                        color: #61666b;
                        max-width: 181px;
                        word-break: break-word;
                        margin-bottom: 15px;
                    }

                    &__link {
                        width: 181px;
                        height: 40px;
                        border: 1px solid #2683ff;
                        border-radius: 6px;
                        background-color: #fff;
                        color: #2683ff;
                        display: flex;
                        align-items: center;
                        justify-content: center;

                        &:hover {
                            background-color: #2683ff;
                            color: #fff;
                        }
                    }
                }
            }
        }

        &__support-info {
            margin-top: 50px;
            font-family: 'font_bold', sans-serif;
            font-size: 15px;
            line-height: 22px;
            text-align: center;
            color: #909ba8;

            &__link {
                text-decoration: underline;
            }
        }
    }

    .uplink-option {
        margin: 0 45px;
    }

    @media screen and (max-width: 1350px) {

        .uplink-option {
            margin: 0 15px;
        }
    }
</style>