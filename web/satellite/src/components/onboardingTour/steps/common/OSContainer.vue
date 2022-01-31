// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="os">
        <div class="os__tabs">
            <p
                class="os__tabs__choice"
                :class="{active: isWindows && !isInstallStep, 'active-install-step': isWindows && isInstallStep}"
                aria-roledescription="windows"
                @click="setIsWindows"
            >
                Windows
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isLinux && !isInstallStep, 'active-install-step': isLinux && isInstallStep}"
                aria-roledescription="linux"
                @click="setIsLinux"
            >
                Linux
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isMacOS && !isInstallStep, 'active-install-step': isMacOS && isInstallStep}"
                aria-roledescription="macos"
                @click="setIsMacOS"
            >
                macOS
            </p>
        </div>
        <slot v-if="isWindows" name="windows" />
        <slot v-if="isLinux" name="linux" />
        <slot v-if="isMacOS" name="macos" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

// @vue/component
@Component
export default class OSContainer extends Vue {
    @Prop({ default: false })
    public readonly isInstallStep: boolean;

    public isWindows = false;
    public isLinux = false;
    public isMacOS = false;

    /**
     * Lifecycle hook after initial render.
     * Checks user's OS.
     */
    public mounted(): void {
        switch (true) {
        case navigator.appVersion.indexOf("Mac") !== -1:
            this.isMacOS = true;
            return;
        case navigator.appVersion.indexOf("Linux") !== -1:
            this.isLinux = true;
            return;
        default:
            this.isWindows = true;
        }
    }

    /**
     * Sets view to windows state.
     */
    public setIsWindows(): void {
        this.isLinux = false;
        this.isMacOS = false;
        this.isWindows = true;
    }

    /**
     * Sets view to linux state.
     */
    public setIsLinux(): void {
        this.isMacOS = false;
        this.isWindows = false;
        this.isLinux = true;
    }

    /**
     * Sets view to macOS state.
     */
    public setIsMacOS(): void {
        this.isLinux = false;
        this.isWindows = false;
        this.isMacOS = true;
    }
}
</script>

<style scoped lang="scss">
    .os {
        font-family: 'font_regular', sans-serif;
        margin: 20px 0;

        &__tabs {
            display: inline-flex;
            align-items: center;
            border-radius: 0 6px 6px 0;
            border-top: 1px solid rgb(230, 236, 241);
            border-left: 1px solid rgb(230, 236, 241);
            border-right: 1px solid rgb(230, 236, 241);
            margin-bottom: -1px;

            &__choice {
                background-color: #f5f7f9;
                font-size: 14px;
                padding: 16px;
                color: #9daab6;
                cursor: pointer;
            }

            &__choice:first-child {
                border-right: 1px solid rgb(230, 236, 241);
            }

            &__choice:last-child {
                border-left: 1px solid rgb(230, 236, 241);
            }
        }
    }

    .active {
        background: rgb(24, 48, 85);
        color: #e6ecf1;
    }

    .active-install-step {
        background: #fff;
        color: #242a31;
    }
</style>
