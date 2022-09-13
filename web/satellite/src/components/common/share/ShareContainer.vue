// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="share-container">
        <ShareButton
            v-for="button of shareButtons"
            :key="button.label"
            :label="button.label"
            :link="button.link"
            :color="button.color"
        />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ShareButton from '@/components/common/share/ShareButton.vue';

export enum ShareOptions {
    Reddit = 'Reddit',
    Facebook = 'Facebook',
    Twitter = 'Twitter',
    HackerNews = 'Hacker News',
    LinkedIn = 'LinkedIn',
    Telegram = 'Telegram',
    WhatsApp = 'WhatsApp',
    Email = 'E-Mail',
}

type ShareButtonConfig = {
    link: string,
    label: ShareOptions,
    color: string,
}

// @vue/component
@Component({
    components: {
        ShareButton,
    },
})
export default class ShareContainer extends Vue {
    @Prop({ default: '' })
    private readonly link: string;

    private readonly ShareOptions = ShareOptions;

    /**
     * Returns share buttons list.
     */
    private get shareButtons(): ShareButtonConfig[] {
        return [
            { label: ShareOptions.Reddit, color: '#5f99cf', link: this.redditLink },
            { label: ShareOptions.Facebook, color: '#3b5998', link: this.facebookLink },
            { label: ShareOptions.Twitter, color: '#55acee', link: this.twitterLink },
            { label: ShareOptions.HackerNews, color: '#f60', link: this.hackernewsLink },
            { label: ShareOptions.LinkedIn, color: '#0077b5', link: this.linkedinLink },
            { label: ShareOptions.Telegram, color: '#54a9eb', link: this.telegramLink },
            { label: ShareOptions.WhatsApp, color: '#25d366', link: this.whatsappLink },
            { label: ShareOptions.Email, color: '#777', link: this.emailLink },
        ];
    }

    /**
     * Return the reddit link to share the current bucket on reddit.
     */
    private get redditLink(): string {
        return `https://reddit.com/submit/?url=${this.link}&resubmit=true&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`;
    }

    /**
     * Return the facebook link to share the current bucket on facebook.
     */
    public get facebookLink(): string {
        return `https://facebook.com/sharer/sharer.php?u=${this.link}`;
    }

    /**
     * Return the twitter link to share the current bucket on twitter.
     */
    public get twitterLink(): string {
        return `https://twitter.com/intent/tweet/?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${this.link}`;
    }

    /**
     * Return the hacker news link to share the current bucket on hacker news.
     */
    public get hackernewsLink(): string {
        return `https://news.ycombinator.com/submitlink?u=${this.link}&t=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`;
    }

    /**
     * Return the linkedin link to share the current bucket on linkedin.
     */
    public get linkedinLink(): string {
        return `https://www.linkedin.com/shareArticle?mini=true&url=${this.link}&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&summary=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&source=${this.link}`;
    }

    /**
     * Return the telegram link to share the current bucket on telegram.
     */
    public get telegramLink(): string {
        return `https://telegram.me/share/url?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${this.link}`;
    }

    /**
     * Return the whatsapp link to share the current bucket on whatsapp.
     */
    public get whatsappLink(): string {
        return `whatsapp://send?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage%20${this.link}`;
    }

    /**
     * Return the email link to share the current bucket through email.
     */
    public get emailLink(): string {
        return `mailto:?subject=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&body=${this.link}`;
    }
}
</script>

<style scoped lang="scss">
    .share-container {
        width: 100%;
        display: flex;
        align-items: center;
        flex-wrap: wrap;
    }
</style>
