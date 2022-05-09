// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-container">
        <WebIcon v-if="isWeb" class="overview-container__img" alt="web" />
        <CLIIcon v-else class="overview-container__img" alt="cli" />
        <h2 class="overview-container__title">{{ title }}</h2>
        <p v-if="isWeb" class="overview-container__enc" aria-roledescription="server-side-encryption-title">Server-side encrypted</p>
        <p v-else class="overview-container__enc" aria-roledescription="end-to-end-encryption-title">End-to-end encrypted</p>
        <p class="overview-container__info">{{ info }}</p>
        <p v-if="isWeb" class="overview-container__encryption-container">
            By using the web browser you are opting in to
            <a
                class="overview-container__encryption-container__link"
                href="https://docs.storj.io/concepts/encryption-key/design-decision-server-side-encryption"
                target="_blank"
                rel="noopener noreferrer"
                aria-roledescription="server-side-encryption-link"
            >server-side encryption</a>.
        </p>
        <p v-else class="overview-container__encryption-container">
            The Uplink CLI uses
            <a
                class="overview-container__encryption-container__link"
                href="https://docs.storj.io/concepts/encryption-key/design-decision-end-to-end-encryption"
                target="_blank"
                rel="noopener noreferrer"
                aria-roledescription="end-to-end-encryption-link"
            >end-to-end</a> encryption for object data, metadata and path data.
        </p>
        <VButton
            :label="buttonLabel"
            width="100%"
            height="64px"
            border-radius="62px"
            is-uppercase="true"
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

// @vue/component
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
    public readonly info: string;
    @Prop({ default: false})
    public readonly isDisabled: boolean;
    @Prop({ default: ''})
    public readonly buttonLabel: string;
    @Prop({ default: () => () => {}})
    public readonly onClick: () => void;
}
</script>

<style scoped lang="scss">
    .overview-container {
        background: #fcfcfc;
        box-shadow: 0 0 32px rgb(0 0 0 / 4%);
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
            margin: 5px 0 10px;
        }

        &__enc {
            font-family: 'font_medium', sans-serif;
            font-size: 14px;
            line-height: 36px;
            letter-spacing: 1px;
            color: #14142b;
            font-weight: 300;
            margin: 0 0 10px;
            text-transform: uppercase;
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

            &__link {
                text-decoration: underline !important;
                text-underline-position: under;

                &:visited {
                    color: #14142a;
                }
            }
        }
    }
</style>
