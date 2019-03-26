// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-button-container" id="accountDropdownButton">
        <div class="account-button-toggle-container" v-on:click="toggleSelection" >
            <!-- background of this div generated and stores in store -->
            <div class="account-button-toggle-container__avatar">
                <!-- First digit of firstName after Registration -->
                <!-- img if avatar was set -->
                <h1>{{avatarLetter}}</h1>
            </div>
            <h1 class="account-button-toggle-container__user-name">{{userName}}</h1>
            <div class="account-button-toggle-container__expander-area">
                <img v-if="!isDropdownShown" src="../../../static/images/register/BlueExpand.svg" />
                <img v-if="isDropdownShown" src="../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <AccountDropdown v-if="isDropdownShown" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import AccountDropdown from './AccountDropdown.vue';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component(
    {
        computed: {
            // May change later
            avatarLetter: function (): string {
                return this.$store.getters.userName.slice(0, 1).toUpperCase();
            },
            userName: function (): string {
                return this.$store.getters.userName;
            },
            isDropdownShown: function (): boolean {
                return this.$store.state.appStateModule.appState.isAccountDropdownShown;
            },
        },
        methods: {
            toggleSelection: function (): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_ACCOUNT);
            }
        },
        components: {
            AccountDropdown
        }
    }
)

export default class AccountButton extends Vue {
}
</script>

<style scoped lang="scss">
    a {
        text-decoration: none;
        outline: none;
    }
    .account-button-container {
        position: relative;
        padding-left: 10px;
        padding-right: 10px;
        background-color: #FFFFFF;
        cursor: pointer;

        &:hover {

            .account-button-toggle-container__user-name {
                opacity: 0.7;
            }
        }
    }

    .account-button-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        width: max-content;
        height: 50px;

        &__user-name {
            margin-left: 12px;
            font-family: 'font_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
            transition: opacity .2s ease-in-out;
        }

        &__avatar {
            width: 40px;
            height: 40px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #E8EAF2;

            h1 {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 23px;
                color: #354049;
            }
        }

        &__expander-area {
            margin-left: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 28px;
            height: 28px;
        }
    }

    @media screen and (max-width: 720px) {

        .account-button-toggle-container {
            &__user-name {
                display: none;
            }
        }
    }
</style>