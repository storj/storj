// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="share-container">
        <ShareButton
            v-for="button of shareButtons"
            :key="button.label"
            :item="button"
        />
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { ShareButtonConfig, ShareOptions } from '@/types/browser';

import ShareButton from '@/components/common/share/ShareButton.vue';

import RedditIcon from '@/../static/images/objects/reddit.svg';
import FacebookIcon from '@/../static/images/objects/facebook.svg';
import TwitterIcon from '@/../static/images/objects/twitter.svg';
import HackerNewsIcon from '@/../static/images/objects/hackerNews.svg';
import LinkedInIcon from '@/../static/images/objects/linkedIn.svg';
import TelegramIcon from '@/../static/images/objects/telegram.svg';
import WhatsAppIcon from '@/../static/images/objects/whatsApp.svg';
import EmailIcon from '@/../static/images/objects/email.svg';

const props = defineProps<{ link: string; }>();

const images: Record<string, string> = {
    [ShareOptions.Reddit]: RedditIcon,
    [ShareOptions.Facebook]: FacebookIcon,
    [ShareOptions.Twitter]: TwitterIcon,
    [ShareOptions.HackerNews]: HackerNewsIcon,
    [ShareOptions.LinkedIn]: LinkedInIcon,
    [ShareOptions.Telegram]: TelegramIcon,
    [ShareOptions.WhatsApp]: WhatsAppIcon,
    [ShareOptions.Email]: EmailIcon,
};

/**
 * Returns share buttons list.
 */
const shareButtons = computed((): ShareButtonConfig[] => {
    return [
        new ShareButtonConfig(
            ShareOptions.Reddit,
            '#5f99cf',
            `https://reddit.com/submit/?url=${props.link}&resubmit=true&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`,
            images[ShareOptions.Reddit],
        ),
        new ShareButtonConfig(
            ShareOptions.Facebook,
            '#3b5998',
            `https://facebook.com/sharer/sharer.php?u=${props.link}`,
            images[ShareOptions.Facebook],
        ),
        new ShareButtonConfig(
            ShareOptions.Twitter,
            '#55acee',
            `https://twitter.com/intent/tweet/?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${props.link}`,
            images[ShareOptions.Twitter],
        ),
        new ShareButtonConfig(
            ShareOptions.HackerNews,
            '#f60',
            `https://news.ycombinator.com/submitlink?u=${props.link}&t=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`,
            images[ShareOptions.HackerNews],
        ),
        new ShareButtonConfig(
            ShareOptions.LinkedIn,
            '#0077b5',
            `https://www.linkedin.com/shareArticle?mini=true&url=${props.link}&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&summary=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&source=${props.link}`,
            images[ShareOptions.LinkedIn],
        ),
        new ShareButtonConfig(
            ShareOptions.Telegram,
            '#54a9eb',
            `https://telegram.me/share/url?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${props.link}`,
            images[ShareOptions.Telegram],
        ),
        new ShareButtonConfig(
            ShareOptions.WhatsApp,
            '#25d366',
            `whatsapp://send?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage%20${props.link}`,
            images[ShareOptions.WhatsApp],
        ),
        new ShareButtonConfig(
            ShareOptions.Email,
            '#777',
            `mailto:?subject=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&body=${props.link}`,
            images[ShareOptions.Email],
        ),
    ];
});
</script>

<style scoped lang="scss">
    .share-container {
        width: 100%;
        display: flex;
        align-items: center;
        flex-wrap: wrap;
    }
</style>
