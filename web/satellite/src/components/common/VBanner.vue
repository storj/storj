// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner-wrap">
        <div class="banner" :class="{link: isLinkActive}" @click="onBannerClick">
            <NotificationSvg />
            <div class="column">
                <p class="banner__text">{{ text }}</p>
                <p class="banner__additional-text">{{ additionalText }}</p>
            </div>
            <ArrowRightIcon class="banner__arrow" v-if="isLinkActive" />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import ArrowRightIcon from '@/../static/images/common/BlueArrowRight.svg';
import NotificationSvg from '@/../static/images/notifications/notification.svg';

/**
 * VBanner is custom banner on top of all pages in Dashboard
 */
@Component({
    components: {
        NotificationSvg,
        ArrowRightIcon,
    },
})
export default class VBanner extends Vue {
    @Prop({default: ''})
    private readonly text: string;
    @Prop({default: ''})
    private readonly additionalText: string;
    @Prop({default: '/'})
    private readonly path: string;

    public get isLinkActive(): boolean {
        return this.$route.path !== this.path;
    }

    public onBannerClick(): void {
        if (!this.isLinkActive) {
            return;
        }

        this.$router.push(this.path);
    }
}
</script>

<style scoped lang="scss">
    .banner-wrap {
        margin: 32px 65px 15px 65px;
    }

    .banner {
        position: relative;
        display: flex;
        align-items: center;
        justify-content: flex-start;
        padding: 20px;
        border-radius: 12px;
        background-color: #d0e3fe;

        &__text {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            font-weight: 500;
            line-height: 19px;
            color: #1e2d42;
            margin: 0;
        }

        &__additional-text {
            font-family: 'font_regular', sans-serif;
            margin: 3px 0 0 0;
            font-size: 14px;
            color: #717e92;
        }

        &__arrow {
            position: absolute;
            top: 50%;
            right: 32px;
            transform: translate(0, -50%);
        }
    }

    .column {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        margin: 0 17px;
    }

    .link {
        cursor: pointer;
    }
</style>
