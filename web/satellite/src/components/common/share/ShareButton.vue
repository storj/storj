// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <a
        class="share-button"
        :href="link"
        target="_blank"
        rel="noopener noreferrer"
        :aria-label="label"
        :style="style"
    >
        <component :is="images[label]" />
        <span>{{ label }}</span>
    </a>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';
import { VueConstructor } from 'vue';

import { ShareOptions } from '@/components/common/share/ShareContainer.vue';

import RedditIcon from '@/../static/images/objects/reddit.svg';
import FacebookIcon from '@/../static/images/objects/facebook.svg';
import TwitterIcon from '@/../static/images/objects/twitter.svg';
import HackerNewsIcon from '@/../static/images/objects/hackerNews.svg';
import LinkedInIcon from '@/../static/images/objects/linkedIn.svg';
import TelegramIcon from '@/../static/images/objects/telegram.svg';
import WhatsAppIcon from '@/../static/images/objects/whatsApp.svg';
import EmailIcon from '@/../static/images/objects/email.svg';

// @vue/component
@Component
export default class ShareButton extends Vue {
    @Prop({ default: '' })
    public readonly label: ShareOptions;
    @Prop({ default: '' })
    public readonly link: string;
    @Prop({ default: '#000' })
    public readonly color: string;

    private readonly images: Record<string, VueConstructor<Vue>> = {
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
     * Returns share button background color.
     */
    public get style(): Record<string, string> {
        return { 'background-color': this.color };
    }
}
</script>

<style scoped lang="scss">
    .share-button {
        display: flex;
        align-items: center;
        text-decoration: none;
        color: #fff;
        margin-right: 1em;
        margin-bottom: 1em;
        border-radius: 5px;
        transition: 25ms ease-out;
        padding: 0.5em 0.75em;
        font-size: 12px;
        font-family: 'font_regular', sans-serif;

        svg {
            width: 12px;
            height: 12px;
            margin-right: 5px;
        }
    }
</style>
