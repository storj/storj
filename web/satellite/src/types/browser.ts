// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component } from 'vue';

import RedditIcon from '@poc/components/icons/share/IconReddit.vue';
import FacebookIcon from '@poc/components/icons/share/IconFacebook.vue';
import TwitterIcon from '@poc/components/icons/share/IconTwitter.vue';
import HackerNewsIcon from '@poc/components/icons/share/IconHackerNews.vue';
import LinkedInIcon from '@poc/components/icons/share/IconLinkedIn.vue';
import TelegramIcon from '@poc/components/icons/share/IconTelegram.vue';
import WhatsAppIcon from '@poc/components/icons/share/IconWhatsApp.vue';
import EmailIcon from '@poc/components/icons/share/IconEmail.vue';

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

export interface ShareButtonConfig {
    color: string;
    getLink: (linksharingURL: string) => string;
    icon: Component;
}

export const SHARE_BUTTON_CONFIGS: Record<ShareOptions, ShareButtonConfig> = {
    [ShareOptions.Reddit]: {
        color: '#5f99cf',
        getLink: url => `https://reddit.com/submit/?url=${url}&resubmit=true&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`,
        icon: RedditIcon,
    },
    [ShareOptions.Facebook]: {
        color: '#3b5998',
        getLink: url => `https://facebook.com/sharer/sharer.php?u=${url}`,
        icon: FacebookIcon,
    },
    [ShareOptions.Twitter]: {
        color: '#55acee',
        getLink: url => `https://twitter.com/intent/tweet/?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${url}`,
        icon: TwitterIcon,
    },
    [ShareOptions.HackerNews]: {
        color: '#f60',
        getLink: url => `https://news.ycombinator.com/submitlink?u=${url}&t=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage`,
        icon: HackerNewsIcon,
    },
    [ShareOptions.LinkedIn]: {
        color: '#0077b5',
        getLink: url => `https://www.linkedin.com/shareArticle?mini=true&url=${url}&title=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&summary=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&source=${url}`,
        icon: LinkedInIcon,
    },
    [ShareOptions.Telegram]: {
        color: '#54a9eb',
        getLink: url => `https://telegram.me/share/url?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&url=${url}`,
        icon: TelegramIcon,
    },
    [ShareOptions.WhatsApp]: {
        color: '#25d366',
        getLink: url => `whatsapp://send?text=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage%20${url}`,
        icon: WhatsAppIcon,
    },
    [ShareOptions.Email]: {
        color: '#777',
        getLink: url => `mailto:?subject=Shared%20using%20Storj%20Decentralized%20Cloud%20Storage&body=${url}`,
        icon: EmailIcon,
    },
};

export enum ShareType {
    File = 'File',
    Folder = 'Folder',
    Bucket = 'Bucket',
}

export enum PreviewType {
    None,
    Text,
    CSV,
    Image,
    Video,
    Audio,
    PDF,
}

export const EXTENSION_PREVIEW_TYPES = new Map<string[], PreviewType>([
    [['txt'], PreviewType.Text],
    [['csv'], PreviewType.CSV],
    [['bmp', 'svg', 'jpg', 'jpeg', 'png', 'ico', 'gif'], PreviewType.Image],
    [['m4v', 'mp4', 'webm', 'mov', 'mkv'], PreviewType.Video],
    [['m4a', 'mp3', 'wav', 'ogg'], PreviewType.Audio],
    [['pdf'], PreviewType.PDF],
]);
