// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-container">
        <WebIcon v-if="isWeb" class="overview-container__img" alt="web" />
        <CLIIcon v-else class="overview-container__img" alt="cli" />
        <p v-if="isWeb" class="overview-container__enc" aria-roledescription="server-side-encryption-title">Server-side encrypted</p>
        <p v-else class="overview-container__enc" aria-roledescription="end-to-end-encryption-title">End-to-end encrypted</p>
        <h2 class="overview-container__title">{{ title }}</h2>
        <p class="overview-container__info">{{ info }}</p>
        <VButton
            :label="buttonLabel"
            width="240px"
            height="48px"
            border-radius="8px"
            :is-disabled="isDisabled"
            :on-press="onClick"
        />
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
            >end-to-end encryption</a> for object data, metadata and path data.
        </p>
    </div>
</template>

<script setup lang="ts">
import VButton from '@/components/common/VButton.vue';

import WebIcon from '@/../static/images/onboardingTour/web.svg';
import CLIIcon from '@/../static/images/onboardingTour/cli.svg';

const props = withDefaults(defineProps<{
    isWeb: boolean;
    title: string;
    info: string;
    isDisabled: boolean;
    buttonLabel: string;
    onClick: () => void;
}>(), {
    isWeb: false,
    title: '',
    info: '',
    isDisabled: false,
    buttonLabel: '',
    onClick: () => {},
});
</script>

<style scoped lang="scss">
.overview-container {
    background: #fcfcfc;
    box-shadow: 0 0 32px rgb(0 0 0 / 4%);
    border-radius: 20px;
    font-family: 'font_regular', sans-serif;
    padding: 52px;
    max-width: 394px;
    display: flex;
    flex-direction: column;
    align-items: center;

    &__img {
        min-height: 83px;
    }

    &__enc {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 36px;
        letter-spacing: 1px;
        color: #14142a;
        margin-bottom: 10px;
        text-transform: uppercase;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 26px;
        line-height: 31px;
        letter-spacing: 1px;
        color: #131621;
        margin-bottom: 14px;
    }

    &__info {
        margin-bottom: 25px;
        font-size: 16px;
        line-height: 21px;
        text-align: center;
        color: #354049;
    }

    &__encryption-container {
        width: calc(100% - 48px);
        padding: 12px 24px;
        background: #fec;
        border-radius: 8px;
        margin-top: 37px;
        font-size: 16px;
        line-height: 24px;
        color: #354049;
        text-align: center;

        &__link {
            text-decoration: underline !important;
            text-underline-position: under;
            color: #354049;

            &:visited {
                color: #354049;
            }
        }
    }
}

@media screen and (max-width: 760px) {

    .overview-container {
        width: 250px;
    }

    .overview-container__title {
        text-align: center;
    }
}
</style>
