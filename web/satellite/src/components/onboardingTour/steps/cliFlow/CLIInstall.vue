// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Install Uplink CLI"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="cli-install">
            <p class="cli-install__msg">Install the Uplink CLI binary for your OS.</p>
            <OSContainer is-install-step="true">
                <template #windows>
                    <div class="cli-install__windows">
                        <h2 class="cli-install__macos__sub-title">
                            1. Download the
                            <a href="https://github.com/storj/storj/releases/latest/download/uplink_windows_amd64.zip">
                                Windows Uplink Binary
                            </a>
                            zip file
                        </h2>
                        <p class="cli-install__windows__msg">
                            2. In the Downloads folder, right-click and select "Extract all".
                        </p>
                        <p class="cli-install__windows__msg">3. Extract to Desktop.</p>
                        <p class="cli-install__windows__msg">
                            4. Once extracted, do not open the file, as it can only be accessed via command line.
                        </p>
                        <p class="cli-install__windows__msg">
                            5. Open
                            <b class="cli-install__windows__msg__bold">Windows PowerShell</b>
                            and continue on to the next step.
                        </p>
                    </div>
                </template>
                <template #linux>
                    <div class="cli-install__linux">
                        <h1 class="cli-install__linux__title">AMD64</h1>
                        <h2 class="cli-install__linux__sub-title">Curl Download</h2>
                        <div class="cli-install__linux__commands">
                            <p class="cli-install__linux__commands__item">
                                curl -L https://github.com/storj/storj/releases/latest/download/uplink_linux_amd64.zip -o uplink_linux_amd64.zip
                            </p>
                            <p class="cli-install__linux__commands__item">
                                unzip -o uplink_linux_amd64.zip
                            </p>
                            <p class="cli-install__linux__commands__item">
                                sudo install uplink /usr/local/bin/uplink
                            </p>
                        </div>
                        <a
                            class="cli-install__linux__link"
                            href="https://github.com/storj/storj/releases/latest/download/uplink_linux_amd64.zip"
                        >
                            Linux AMD64 Uplink Binary
                        </a>
                        <h1 class="cli-install__linux__title margin-top">ARM</h1>
                        <h2 class="cli-install__linux__sub-title">Curl Download</h2>
                        <div class="cli-install__linux__commands">
                            <p class="cli-install__linux__commands__item">
                                curl -L https://github.com/storj/storj/releases/latest/download/uplink_linux_arm.zip -o uplink_linux_arm.zip
                            </p>
                            <p class="cli-install__linux__commands__item">
                                unzip -o uplink_linux_arm.zip
                            </p>
                            <p class="cli-install__linux__commands__item">
                                sudo install uplink /usr/local/bin/uplink
                            </p>
                        </div>
                        <a
                            class="cli-install__linux__link"
                            href="https://github.com/storj/storj/releases/latest/download/uplink_linux_arm.zip"
                        >
                            Linux ARM Uplink Binary
                        </a>
                    </div>
                </template>
                <template #macos>
                    <div class="cli-install__macos">
                        <h2 class="cli-install__macos__sub-title">Curl Download</h2>
                        <div class="cli-install__macos__commands">
                            <p class="cli-install__macos__commands__item">
                                curl -L https://github.com/storj/storj/releases/latest/download/uplink_darwin_amd64.zip -o uplink_darwin_amd64.zip
                            </p>
                            <p class="cli-install__macos__commands__item">
                                unzip -o uplink_darwin_amd64.zip
                            </p>
                            <p class="cli-install__macos__commands__item">
                                sudo install uplink /usr/local/bin/uplink
                            </p>
                        </div>
                        <a
                            class="cli-install__macos__link"
                            href="https://github.com/storj/storj/releases/latest/download/uplink_darwin_amd64.zip"
                        >
                            macOS Uplink Binary
                        </a>
                    </div>
                </template>
            </OSContainer>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import OSContainer from "@/components/onboardingTour/steps/common/OSContainer.vue";

import Icon from '@/../static/images/onboardingTour/cliSetupStep.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
    }
})
export default class CLIInstall extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.APIKey)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLISetup)).path);
    }
}
</script>

<style scoped lang="scss">
    .cli-install {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }

        &__macos,
        &__linux,
        &__windows {
            border: 1px solid rgb(230, 236, 241);
            display: block;
            padding: 24px;
            background: rgb(255, 255, 255);
            border-radius: 0 6px 6px;

            &__title {
                font-size: 20px;
                margin-bottom: 24px;
            }

            &__sub-title {
                font-size: 16px;
            }

            &__title,
            &__sub-title {
                line-height: 1.5;
                font-family: 'font_medium', sans-serif;
            }

            &__commands {
                color: rgb(230, 236, 241);
                margin: 32px 0;
                padding: 24px;
                overflow: auto;
                font-size: 14px;
                background: rgb(24, 48, 85);

                &__item {
                    white-space: nowrap;
                }
            }

            &__link {
                color: rgb(55, 111, 255);
            }

            &__msg {
                font-size: 15px;
                line-height: 1.625;
                margin-top: 20px;

                &__bold {
                    font-family: 'font_medium', sans-serif;
                }
            }
        }
    }

    .margin-top {
        margin-top: 24px;
    }
</style>
