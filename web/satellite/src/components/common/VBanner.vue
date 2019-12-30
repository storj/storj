// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="banner">
        <NotificationSvg />
        <div class="column">
            <p class="banner__text">{{ text }}</p>
            <p class="banner__additional-text">{{ additionalText }}</p>
        </div>
        <router-link :to="path" class="banner__link" v-if="isLinkActive">
            <ArrowRightIcon />
        </router-link>
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
}
</script>

<style scoped lang="scss">
    .banner {
        position: relative;
        display: flex;
        align-items: center;
        justify-content: flex-start;
        padding: 20px 20px 20px 20px;
        border-radius: 12px;
        background-color: #d0e3fe;
        margin: 32px 65px 15px 65px;

        &__text {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            font-weight: 500;
            line-height: 19px;
            margin: 0;
        }

        &__additional-text {
            font-family: 'font_regular', sans-serif;
            margin: 3px 0 0 0;
            font-size: 14px;
            color: #717e92;
        }

        &__link {
            position: absolute;
            top: 50%;
            right: 32px;
            transform: translate(0, -50%);
            display: flex;
            align-items: center;
            justify-content: center;
            width: 32px;
            height: 32px;
        }
    }

    .column {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        justify-content: center;
        margin: 0 17px;
    }
</style>
