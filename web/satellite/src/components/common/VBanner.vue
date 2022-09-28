// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div
        v-if="isShown && bannerWidth > 0"
        class="notification-wrap"
        :class="{ warning: severity === 'warning', critical: severity === 'critical' }"
        :style="bannerStyle"
        @click="onClick"
    >
        <InfoIcon class="notification-wrap__icon" />
        <div class="notification-wrap__text">
            <slot name="text" />
        </div>
        <CloseIcon class="notification-wrap__close" @click="isShown = false" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Watch } from 'vue-property-decorator';

import Resizable from '@/components/common/Resizable.vue';

import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

// @vue/component
@Component({
    components: {
        CloseIcon,
        InfoIcon,
    },
})
export default class VBanner extends Resizable {
    @Prop({ default: 'info' })
    private readonly severity: 'info' | 'warning' | 'critical';
    @Prop({ default: () => () => {} })
    public readonly onClick: () => void;
    @Prop({ default: () => {} })
    private readonly dashboardRef: HTMLElement;

    public isShown = true;
    public bannerWidth = 0;
    public resizeObserver?: ResizeObserver;

    public mounted(): void {
        this.resizeObserver = new ResizeObserver(this.onBannerResize);

        if (this.dashboardRef) {
            this.resizeObserver?.observe(this.dashboardRef);
            this.onBannerResize();
        }
    }

    public beforeUnmount(): void {
        this.resizeObserver?.unobserve(this.dashboardRef);
    }

    @Watch('dashboardRef')
    public setResizable(): void {
        this.resizeObserver?.observe(this.dashboardRef);
    }

    @Watch('dashboardRef')
    public onBannerResize(): void {
        this.bannerWidth = this.dashboardRef.offsetWidth;
    }

    public get bannerStyle(): string {
        const margin = this.isMobile ? 30 : 60;

        return `width: ${this.bannerWidth - margin}px`;
    }
}
</script>

<style scoped lang="scss">
.notification-wrap {
    position: fixed;
    right: 30px;
    top: 5rem;
    z-index: 9998;
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.375rem;
    font-family: 'font_regular', sans-serif;
    background-color: var(--c-info);
    border: 1px solid var(--c-info-border);
    border-radius: 10px;
    box-shadow: 0 7px 20px rgba(0 0 0 / 15%);
    box-sizing: border-box;

    &__icon {
        flex-shrink: 0;
        margin-right: 1.375rem;

        :deep(path) {
            fill: var(--c-info-icon);
        }
    }

    &.warning {
        background-color: var(--c-warning);
        border: 1px solid var(--c-warning-border);

        :deep(.icon-path) {
            fill: var(--c-warning-icon) !important;
        }
    }

    &.critical {
        background-color: var(--c-critical);
        border: 1px solid var(--c-critical-border);

        :deep(.icon-path) {
            fill: var(--c-critical-icon);
        }
    }

    &__text {
        width: 100%;
        text-align: left;
        word-break: normal;
        font-size: 1rem;
        line-height: 1.625rem;
        display: flex;
        align-items: center;
        justify-content: space-between;
    }

    &__close {
        margin-left: 2.375rem;
        cursor: pointer;
    }
}

.bold {
    font-family: 'font_bold', sans-serif;
}

.medium {
    font-family: 'font_medium', sans-serif;
}

.link {
    color: black;
    text-decoration: underline !important;
}

@media screen and (max-width: 500px) {

    .notification-wrap {
        right: 15px;
    }
}
</style>
