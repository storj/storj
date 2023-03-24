// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="os">
        <div class="os__tabs">
            <p
                class="os__tabs__choice"
                :class="{active: isWindows && !isInstallStep, 'active-install-step': isWindows && isInstallStep}"
                aria-roledescription="windows"
                @click="setTab(OnboardingOS.WINDOWS)"
            >
                Windows
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isLinux && !isInstallStep, 'active-install-step': isLinux && isInstallStep}"
                aria-roledescription="linux"
                @click="setTab(OnboardingOS.LINUX)"
            >
                Linux
            </p>
            <p
                class="os__tabs__choice"
                :class="{active: isMacOS && !isInstallStep, 'active-install-step': isMacOS && isInstallStep}"
                aria-roledescription="macos"
                @click="setTab(OnboardingOS.MAC)"
            >
                macOS
            </p>
        </div>
        <slot v-if="isWindows" :name="OnboardingOS.WINDOWS" />
        <slot v-if="isLinux" :name="OnboardingOS.LINUX" />
        <slot v-if="isMacOS" :name="OnboardingOS.MAC" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { OnboardingOS } from '@/types/common';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

// @vue/component
@Component
export default class OSContainer extends Vue {
    @Prop({ default: false })
    public readonly isInstallStep: boolean;

    public osSelected = OnboardingOS.WINDOWS;
    // This is done to fix "non-reactive property issue" to be able to use enum in template.
    public OnboardingOS = OnboardingOS;

    /**
     * Lifecycle hook after initial render.
     * Checks user's OS.
     */
    public mounted(): void {
        if (this.storedOsSelected) {
            this.setTab(this.storedOsSelected);

            return;
        }

        switch (true) {
        case navigator.appVersion.indexOf('Mac') !== -1:
            this.setTab(OnboardingOS.MAC);
            return;
        case navigator.appVersion.indexOf('Linux') !== -1:
            this.setTab(OnboardingOS.LINUX);
            return;
        default:
            this.setTab(OnboardingOS.WINDOWS);
        }
    }

    /**
     * Sets view to given state.
     */
    public setTab(os: OnboardingOS): void {
        this.osSelected = os;
        this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_OS, os);
    }

    /**
     * Returns selected os from store.
     */
    public get storedOsSelected(): OnboardingOS | null {
        return this.$store.state.appStateModule.viewsState.onbSelectedOs;
    }

    /**
     * Indicates if mac tab is selected.
     */
    private get isMacOS(): boolean {
        return this.osSelected === OnboardingOS.MAC;
    }

    /**
     * Indicates if linux tab is selected.
     */
    private get isLinux(): boolean {
        return this.osSelected === OnboardingOS.LINUX;
    }

    /**
     * Indicates if windows tab is selected.
     */
    private get isWindows(): boolean {
        return this.osSelected === OnboardingOS.WINDOWS;
    }
}
</script>

<style scoped lang="scss">
    .os {
        font-family: 'font_regular', sans-serif;
        margin-top: 20px;
        width: 100%;

        &__tabs {
            display: inline-flex;
            align-items: center;
            border-radius: 0 6px 6px 0;
            border-top: 1px solid rgb(230 236 241);
            border-left: 1px solid rgb(230 236 241);
            border-right: 1px solid rgb(230 236 241);
            margin-bottom: -1px;

            &__choice {
                background-color: #f5f7f9;
                font-size: 14px;
                padding: 16px;
                color: #9daab6;
                cursor: pointer;
            }

            &__choice:first-child {
                border-right: 1px solid rgb(230 236 241);
            }

            &__choice:last-child {
                border-left: 1px solid rgb(230 236 241);
            }
        }
    }

    .active {
        background: rgb(24 48 85);
        color: #e6ecf1;
    }

    .active-install-step {
        background: #fff;
        color: #242a31;
    }
</style>
