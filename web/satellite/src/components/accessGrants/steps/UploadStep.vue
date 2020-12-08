// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="upload-data">
        <h1 class="upload-data__title">Upload Data</h1>
        <p class="upload-data__sub-title">
            From here, you’ll set up Tardigrade to store data for your project using our S3 Gateway, Uplink CLI, or
            select from our growing library of connectors to build apps on Tardigrade.
        </p>
        <div class="upload-data__docs-area" :class="{ justify: !isUplinkSectionEnabled }">
            <div class="upload-data__docs-area__option" :class="{ margin: !isUplinkSectionEnabled }">
                <h2 class="upload-data__docs-area__option__title">
                    Migrate Data from your Existing AWS buckets
                </h2>
                <img src="@/../static/images/accessGrants/s3.png" alt="s3 gateway image">
                <h3 class="upload-data__docs-area__option__sub-title">
                    S3 Gateway
                </h3>
                <p class="upload-data__docs-area__option__info">
                    Make the switch with Tardigrade’s S3 Gateway.
                </p>
                <a
                    class="upload-data__docs-area__option__link"
                    href="https://documentation.tardigrade.io/api-reference/s3-gateway"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    S3 Gateway Docs
                </a>
            </div>
            <div class="upload-data__docs-area__option uplink-option" v-if="isUplinkSectionEnabled">
                <h2 class="upload-data__docs-area__option__title">
                    Upload Data from Your Local Environment
                </h2>
                <img src="@/../static/images/accessGrants/uplinkcli.png" alt="uplink cli image">
                <h3 class="upload-data__docs-area__option__sub-title">
                    Uplink CLI
                </h3>
                <p class="upload-data__docs-area__option__info">
                    Start uploading data from the command line.
                </p>
                <a
                    class="upload-data__docs-area__option__link"
                    href="https://documentation.tardigrade.io/getting-started/uploading-your-first-object/set-up-uplink-cli"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Uplink CLI Docs
                </a>
            </div>
            <div class="upload-data__docs-area__option">
                <h2 class="upload-data__docs-area__option__title">
                    Use Tardigrade for your app’s storage layer
                </h2>
                <img src="@/../static/images/accessGrants/connectors.png" alt="connectors image">
                <h3 class="upload-data__docs-area__option__sub-title">
                    App Connectors
                </h3>
                <p class="upload-data__docs-area__option__info">
                    Integrate Tardigrade into your existing stack.
                </p>
                <a
                    class="upload-data__docs-area__option__link"
                    href="https://tardigrade.io/connectors/"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    App Connectors
                </a>
            </div>
        </div>
        <VButton
            class="upload-data__button"
            label="Close"
            width="238px"
            height="48px"
            :on-press="onCloseClick"
        />
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
export default class UploadStep extends Vue {
    public isUplinkSectionEnabled: boolean = true;

    /**
     * Lifecycle hook after initial render.
     * Sets uplink section visibility from props value.
     */
    public mounted(): void {
        if (!this.$route.params.isUplinkSectionEnabled) {
            this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
        }

        this.$route.params.isUplinkSectionEnabled === 'true' ? this.isUplinkSectionEnabled = true : this.isUplinkSectionEnabled = false;
    }

    /**
     * Holds on close button click logic.
     * Redirects to access grants page.
     */
    public onCloseClick(): void {
        this.$router.push(RouteConfig.AccessGrants.path);
    }
}
</script>

<style scoped lang="scss">
    .upload-data {
        padding: 60px;
        display: flex;
        flex-direction: column;
        align-items: center;
        background-color: rgba(255, 255, 255, 0.4);
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #1b2533;
            margin: 0 0 25px 0;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 24px;
            color: #000;
            margin-bottom: 35px;
            text-align: center;
            word-break: break-word;
            max-width: 680px;
        }

        &__docs-area {
            display: flex;
            align-items: center;
            justify-content: space-between;
            width: 100%;
            margin-bottom: 50px;

            &__option {
                padding: 30px;
                display: flex;
                flex-direction: column;
                align-items: center;
                border: 1px solid rgba(144, 155, 168, 0.4);
                border-radius: 8px;

                &__title {
                    font-family: 'font_bold', sans-serif;
                    font-size: 10px;
                    line-height: 15px;
                    text-align: center;
                    letter-spacing: 0.05em;
                    text-transform: uppercase;
                    color: #1b2533;
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
                    font-family: 'font_bold', sans-serif;
                    width: 181px;
                    height: 40px;
                    border: 1px solid #0068dc;
                    border-radius: 6px;
                    background-color: #fff;
                    color: #0068dc;
                    display: flex;
                    align-items: center;
                    justify-content: center;

                    &:hover {
                        background-color: #0068dc;
                        color: #fff;
                    }
                }
            }
        }
    }

    .uplink-option {
        margin: 0 25px;
    }

    .justify {
        justify-content: center;
    }

    .margin {
        margin-right: 25px;
    }
</style>