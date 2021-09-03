// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Access Grant Generated"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="generated">
            <p class="generated__msg">
                Your first access grant is created. Now copy and save the Access Grant, as it will only appear once.
            </p>
            <div class="generated__download">
                <p class="generated__download__label" @click="onDownloadClick">Download as a text file</p>
                <VInfo
                    class="generated__download__info-button"
                    title="Download the access grant"
                >
                    <template #icon>
                        <InfoIcon class="generated__download__info-button__image" />
                    </template>
                    <template #message>
                        <p class="generated__download__info-button__message">
                            This will make a text file with the access grant, so that you can easily import it later
                            into the Uplink CLI.
                        </p>
                    </template>
                </VInfo>
            </div>
            <h3 class="generated__label">Access Grant</h3>
            <ValueWithCopy label="Access Grant" :value="accessGrant" />
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { Download } from "@/utils/download";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import ValueWithCopy from "@/components/onboardingTour/steps/common/ValueWithCopy.vue";
import VInfo from "@/components/common/VInfo.vue";

import Icon from "@/../static/images/onboardingTour/generatedStep.svg";
import InfoIcon from "@/../static/images/common/greyInfo.svg";

// @vue/component
@Component({
    components: {
        Icon,
        CLIFlowContainer,
        ValueWithCopy,
        VInfo,
        InfoIcon,
    }
})
export default class GeneratedAG extends Vue {
    /**
     * Lifecycle hook before initial render.
     * Redirects to encrypt your data step if there is no AG to show.
     */
    public async beforeMount(): Promise<void> {
        if (!this.accessGrant) {
            await this.onBackClick();
        }
    }

    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.EncryptYourData)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
    }

    /**
     * Holds on download access grant button click logic.
     * Downloads a file with the access called access-grant-<timestamp>.key
     */
    public onDownloadClick(): void {
        const ts = new Date();
        const filename = 'access-grant-' + ts.toJSON() + '.txt';

        Download.file(this.accessGrant, filename);

        this.$notify.success('Access Grant was downloaded successfully');
    }

    /**
     * Returns AG from store.
     */
    public get accessGrant(): string {
        return this.$store.state.accessGrantsModule.onboardingAccessGrant;
    }
}
</script>

<style scoped lang="scss">
    .generated {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }

        &__label {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin-bottom: 20px;
        }

        &__download {
            display: flex;
            align-items: center;
            justify-content: flex-end;
            margin-top: 20px;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 19px;
                color: #0068dc;
                cursor: pointer;
            }

            &__info-button {
                margin-left: 10px;
                max-height: 18px;

                &__image {
                    cursor: pointer;
                }

                &__message {
                    color: #586c86;
                    font-family: 'font_regular', sans-serif;
                    font-size: 12px;
                    line-height: 21px;
                }
            }
        }
    }

    ::v-deep .info__box__message {
        min-width: 345px;

        &__title {
            margin-bottom: 5px;
        }
    }
</style>
