// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="settings-dropdown">
        <div class="settings-dropdown__choice" @click.prevent.stop="onAccountSettingsClick">
            <div class="settings-dropdown__choice__mark-container">
                <SelectionIcon
                    v-if="isAccountSettingsPage"
                    class="settings-dropdown__choice__mark-container__image"
                />
            </div>
            <p class="settings-dropdown__choice__label" :class="{ active: isAccountSettingsPage }">
                Account Settings
            </p>
        </div>
        <div class="settings-dropdown__choice" @click.prevent.stop="onBillingClick">
            <div class="settings-dropdown__choice__mark-container">
                <SelectionIcon
                    v-if="isBillingPage"
                    class="settings-dropdown__choice__mark-container__image"
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

import SelectionIcon from '@/../static/images/header/selection.svg';

import { RouteConfig } from '@/router';

@Component({
    components: {
        SelectionIcon,
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
    .settings-dropdown {
        position: absolute;
        left: 0;
        top: 50px;
        z-index: 1120;
        box-shadow: 0 20px 34px rgba(10, 27, 44, 0.28);
        border-radius: 6px;
        background-color: #fff;
        padding: 6px 0;

        &__choice {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            padding: 0 25px;
            min-width: 255px;
            background-color: #fff;
            border-radius: 6px;
            font-family: 'font_regular', sans-serif;

            &__label {
                margin: 12px 0;
                font-size: 14px;
                line-height: 20px;
                color: #7e8b9c;
            }

            &:hover {
                background-color: #f2f2f6;

                .settings-dropdown__choice__label {
                    color: #354049;
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
