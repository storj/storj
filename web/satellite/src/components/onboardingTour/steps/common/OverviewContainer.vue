// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-container">
        <WebIcon v-if="isWeb" class="overview-container__img" alt="web" />
        <CLIIcon v-else class="overview-container__img" alt="cli" />
        <h2 class="overview-container__title">{{ title }}</h2>
        <p class="overview-container__enc">{{ encryption }}</p>
        <p class="overview-container__info">{{ info }}</p>
        <p class="overview-container__encryption-container">{{ encryptionContainer }}</p>
        <VButton
            :label="buttonLabel"
            width="100%"
            height="64px"
            border-radius="62px"
            :is-disabled="isDisabled"
            :on-press="onClick"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';
import WebIcon from '@/../static/images/onboardingTour/web.svg';
import CLIIcon from '@/../static/images/onboardingTour/cli.svg';

@Component({
    components: {
        VButton,
        WebIcon,
        CLIIcon
    },
})
export default class OverviewContainer extends Vue {
    @Prop({ default: false})
    public readonly isWeb: boolean;
    @Prop({ default: ''})
    public readonly title: string;
    @Prop({ default: ''})
    public readonly encryption: string;
    @Prop({ default: ''})
    public readonly info: string;
    @Prop({ default: '' })
    public readonly encryptionContainer: string;
    @Prop({ default: false})
    public readonly isDisabled: boolean;
    @Prop({ default: ''})
    public readonly buttonLabel: string;
    @Prop({ default: () => false})
    public readonly onClick: () => void;
}
</script>

<style scoped lang="scss">
    .overview-container {
        background: #fcfcfc;
        box-shadow: 0 0 32px rgba(0, 0, 0, 0.04);
        border-radius: 20px;
        font-family: 'font_regular', sans-serif;
        padding: 48px;
        max-width: 396px;

        &__img {
            min-height: 90px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 36px;
            line-height: 36px;
            letter-spacing: 1px;
            color: #14142b;
            margin: 5px 0 10px 0;
        }

        &__enc {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 36px;
            letter-spacing: 1px;
            color: #14142b;
            font-weight: 300;
            margin: 0 0 10px 0;
        }

        &__info {
            font-size: 14px;
            line-height: 22px;
            letter-spacing: 0.75px;
            color: #14142a;
            margin-bottom: 30px;
        }

        &__encryption-container {
            width: calc(100% - 40px);
            padding: 20px;
            border: 1px solid #e6e9ef;
            border-radius: 20px;
            margin-bottom: 30px;
            font-size: 14px;
            line-height: 20px;
            letter-spacing: 0.75px;
            color: #14142a;
        }
    }
</style>
