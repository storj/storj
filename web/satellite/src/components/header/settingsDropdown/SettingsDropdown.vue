// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="settings-dropdown">
        <div class="settings-dropdown__choice" @click.prevent.stop="onAccountSettingsClick">
            <div class="settings-dropdown__choice__mark-container">
                <SettingsIcon
                    class="settings-dropdown__choice__mark-container__image"
                    :class="{ 'image-active': isAccountSettingsPage }"
                />
            </div>
            <p class="settings-dropdown__choice__label" :class="{ active: isAccountSettingsPage }">
                Account Settings
            </p>
        </div>
        <div class="settings-dropdown__choice" @click.prevent.stop="onBillingClick">
            <div class="settings-dropdown__choice__mark-container">
                <BillingIcon
                    class="settings-dropdown__choice__mark-container__image"
                    :class="{ 'image-active': isBillingPage }"
                />
            </div>
            <p class="settings-dropdown__choice__label" :class="{ active: isBillingPage }">
                Billing
            </p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BillingIcon from '@/../static/images/header/billing.svg';
import SettingsIcon from '@/../static/images/header/settings.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        BillingIcon,
        SettingsIcon,
    },
})
export default class SettingsDropdown extends Vue {
    /**
     * Indicates if current route is account settings page.
     */
    public get isAccountSettingsPage(): boolean {
        return this.$route.name === RouteConfig.Settings.name;
    }

    /**
     * Indicates if current route is billing page.
     */
    public get isBillingPage(): boolean {
        return this.$route.name === RouteConfig.Billing.name;
    }

    /**
     * Redirects to account settings page.
     */
    public onAccountSettingsClick(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Settings).path);
        this.closeDropdown();
    }

    /**
     * Redirects to billing page.
     */
    public onBillingClick(): void {
        this.$router.push(RouteConfig.Account.with(RouteConfig.Billing).path);
        this.closeDropdown();
    }

    /**
     * Closes dropdown.
     */
    private closeDropdown(): void {
        this.$emit('close');
    }
}
</script>

<style scoped lang="scss">
    .image-active {

        .settings-svg-path,
        .billing-svg-path {
            fill: #0068dc;
        }
    }

    .settings-dropdown {
        position: absolute;
        left: 0;
        top: 50px;
        z-index: 1120;
        box-shadow: 0 20px 34px rgba(10, 27, 44, 0.28);
        border-radius: 6px;
        background-color: #f5f6fa;
        padding: 6px 0;

        &__choice {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            padding: 0 25px;
            min-width: 205px;
            background-color: #f5f6fa;
            border-radius: 6px;
            font-family: 'font_regular', sans-serif;

            &__label {
                margin: 8px 0 14px 10px;
                font-size: 14px;
                line-height: 20px;
                color: #1b2533;
            }

            &:hover {
                font-family: 'font_bold', sans-serif;

                .settings-svg-path,
                .billing-svg-path {
                    fill: #0068dc;
                }
            }

            &__mark-container {
                width: 10px;
                margin-right: 12px;

                &__image {
                    object-fit: cover;
                }
            }
        }
    }

    .active {
        font-family: 'font_bold', sans-serif;
    }
</style>
