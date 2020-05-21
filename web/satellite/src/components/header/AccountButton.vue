// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-button-container" id="accountDropdownButton">
        <div class="account-button-toggle-container" @click="toggleSelection">
            <div class="account-button-toggle-container__avatar" :class="{ 'expanded-background': isDropdownShown }">
                <h1 class="account-button-toggle-container__avatar__letter" :class="{ 'expanded-font-color': isDropdownShown }">
                    {{ avatarLetter }}
                </h1>
            </div>
            <div class="account-button-toggle-container__expander-area">
                <ExpandIcon
                    v-if="!isDropdownShown"
                    alt="Arrow down (expand)"
                />
                <HideIcon
                    v-if="isDropdownShown"
                    alt="Arrow up (hide)"
                />
            </div>
        </div>
        <AccountDropdown v-if="isDropdownShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HideIcon from '@/../static/images/common/ArrowHide.svg';
import ExpandIcon from '@/../static/images/common/BlackArrowExpand.svg';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

import AccountDropdown from './AccountDropdown.vue';

@Component({
    components: {
        AccountDropdown,
        ExpandIcon,
        HideIcon,
    },
})
export default class AccountButton extends Vue {
    /**
     * Toggles account dropdown.
     */
    public toggleSelection(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
    }

    /**
     * Returns first letter of user`s name.
     */
    public get avatarLetter(): string {
        return this.$store.getters.userName.slice(0, 1).toUpperCase();
    }

    /**
     * Indicates if account dropdown should render.
     */
    public get isDropdownShown(): boolean {
        return this.$store.state.appStateModule.appState.isAccountDropdownShown;
    }
}
</script>

<style scoped lang="scss">
    .account-button-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: max-content;
        height: 50px;

        &__avatar {
            width: 40px;
            height: 40px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #e8eaf2;

            &__letter {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 23px;
                color: #354049;
            }
        }

        &__expander-area {
            margin-left: 9px;
            display: flex;
            align-items: center;
            justify-content: center;
        }
    }

    .account-button-container {
        position: relative;
        background-color: #fff;
        cursor: pointer;
    }

    .expanded-background {
        background-color: #2582ff;
    }

    .expanded-font-color {
        color: #fff;
    }

    @media screen and (max-width: 1280px) {

        .account-button-toggle-container {

            &__expander-area {
                display: none;
            }
        }
    }
</style>
