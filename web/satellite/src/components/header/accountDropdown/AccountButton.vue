// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-button">
        <div
            class="account-button__container"
            @click.stop="toggleDropdown"
        >
            <div class="account-button__container__avatar">
                <h1 class="account-button__container__avatar__letter">
                    {{ avatarLetter }}
                </h1>
            </div>
        </div>
        <AccountDropdown
            v-if="isDropdownShown"
            v-click-outside="closeDropdown"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import AccountDropdown from './AccountDropdown.vue';

@Component({
    components: {
        AccountDropdown,
    },
})
export default class AccountButton extends Vue {
    /**
     * Returns first letter of user`s name.
     */
    public get avatarLetter(): string {
        return this.$store.getters.userName.slice(0, 1).toUpperCase();
    }

    /**
     * Indicates if account dropdown is shown.
     */
    public get isDropdownShown(): string {
        return this.$store.state.appStateModule.appState.isAccountDropdownShown;
    }

    /**
     * Toggles account dropdown's visibility.
     */
    public toggleDropdown(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
    }

    /**
     * Closes account dropdown.
     */
    public closeDropdown(): void {
        if (!this.isDropdownShown) return;

        this.$store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS);
    }
}
</script>

<style scoped lang="scss">
    .account-button {
        background-color: #fff;
        position: relative;

        &__container {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            width: max-content;
            height: 50px;

            &__avatar {
                width: 40px;
                height: 40px;
                border-radius: 20px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #2582ff;
                cursor: pointer;

                &__letter {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #fff;
                }
            }
        }
    }
</style>
