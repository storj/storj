// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Ready to upload"
    >
        <template #content class="upload-object">
            <p class="upload-object__msg">
                Here is an example image you can use for your first upload. Just right-click on it and save as
                <b class="upload-object__msg__bold">cheesecake.jpg</b>
                to your
                <b class="upload-object__msg__bold">Desktop.</b>
            </p>
            <Icon class="upload-object__icon" />
            <p class="upload-object__msg">
                Now to upload the photo, use the copy command.
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe cp <FILE_PATH> sj://cakes" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink cp ~/Desktop/cheesecake.jpg sj://cakes" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink cp ~/Desktop/cheesecake.jpg sj://cakes" />
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
import TabWithCopy from "@/components/onboardingTour/steps/common/TabWithCopy.vue";

import Icon from "@/../static/images/onboardingTour/uploadObjectStep.svg";

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        OSContainer,
        TabWithCopy,
        Icon,
    }
})
export default class UploadObject extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ListObject)).path);
    }
}
</script>

<style scoped lang="scss">
    .upload-object {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }

        &__icon {
            margin: 20px 0 40px 0;
        }
    }

    ::v-deep .flow-container__title {
        margin-top: 0;
    }
</style>
