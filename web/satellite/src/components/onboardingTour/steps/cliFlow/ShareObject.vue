// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Share a link"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="share-object">
            <div class="share-object__msg">
                You can generate a shareable URL and view the geographic distribution of your object via the Link
                Sharing Service. Run the
                <b class="share-object__msg__bold">uplink share --url</b>
                command.
                <div class="absolute">
                    <VInfo
                        class="share-object__msg__info-button"
                        title="Check out the documentation for more info"
                        is-clickable="true"
                    >
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="share-object__msg__info-button__message">
                                See
                                <a
                                    class="share-object__msg__info-button__message__link"
                                    href="https://docs.storj.io/dcs/api-reference/uplink-cli/share-command#link-sharing"
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    here
                                </a>
                                for specifications on how to select an auth region and restrict the
                                <b class="share-object__msg__info-button__message__bold">uplink share --url</b>
                                command.
                            </p>
                        </template>
                    </VInfo>
                </div>
            </div>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe share --url sj://cakes/cheesecake.jpg" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink share --url sj://cakes/cheesecake.jpg" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink share --url sj://cakes/cheesecake.jpg" />
                </template>
            </OSContainer>
            <p class="share-object__msg">
                Copy the URL that is returned by the
                <b class="share-object__msg__bold">uplink share --url</b>
                command and paste into your browser window.
            </p>
            <p class="share-object__msg margin-top">
                You will see your file and a map with real distribution of your files' pieces uploaded to the network.
                You can share it with anyone you'd like.
            </p>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import OSContainer from "@/components/onboardingTour/steps/common/OSContainer.vue";
import TabWithCopy from "@/components/onboardingTour/steps/common/TabWithCopy.vue";
import VInfo from "@/components/common/VInfo.vue";

import Icon from "@/../static/images/onboardingTour/listObjectStep.svg";
import InfoIcon from "@/../static/images/common/greyInfo.svg";

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
        TabWithCopy,
        VInfo,
        InfoIcon,
    }
})
export default class ShareObject extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.DownloadObject)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.SuccessScreen)).path);
    }
}
</script>

<style scoped lang="scss">
    .share-object {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
            position: relative;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }

            &__info-button {
                max-width: 18px;

                &__message {
                    color: #586c86;
                    font-size: 12px;
                    line-height: 21px;

                    &__link {
                        color: #0068dc;
                        font-family: 'font_medium', sans-serif;
                        text-decoration: underline !important;

                        &:visited {
                            color: #0068dc;
                        }
                    }

                    &__bold {
                        font-family: 'font_medium', sans-serif;
                    }
                }
            }
        }
    }

    .margin-top {
        margin-top: 20px;
    }

    .absolute {
        position: absolute;
        bottom: -2px;
        right: -4px;
    }

    ::v-deep .info__box {
        top: calc(100% - 24px);

        &__message {
            min-width: 360px;
        }
    }
</style>
