// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-button-container" >
        <div class="account-button-toggle-container" v-on:click="toggleSelection" >
            <!-- background of this div generated and stores in store -->
            <div class="account-button-toggle-container__avatar" :style="style">
                <!-- First digit of firstName after Registration -->
                <!-- img if avatar was set -->
                <h1>{{avatarLetter}}</h1>
            </div>
            <h1 class="account-button-toggle-container__user-name">{{userName}}</h1>
            <div class="account-button-toggle-container__expander-area">
                <img v-if="!isChoiceShown" src="../../../../static/images/register/BlueExpand.svg" />
                <img v-if="isChoiceShown" src="../../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <AccountDropdown v-if="isChoiceShown" @onClose="toggleSelection" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import AccountDropdown from "./AccountDropdown.vue";

@Component(
    { 
        data: function() {
            return {
                isChoiceShown: false
            }
        },
        computed: {
            style: function() : object {
                //color from $store
				return { background: "#95D486" }
            },
            // may change later
            avatarLetter: function() : string {
                return this.$store.getters.userName.slice(0,1).toUpperCase();
            },
            userName: function (): string {
                return this.$store.getters.userName;

            }
        },
        methods: {
            toggleSelection: function() : void {
                this.$data.isChoiceShown = !this.$data.isChoiceShown;
            }
        },
        components: {
            AccountDropdown
        }
    }
)

export default class AccountButton extends Vue {}
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
            font-family: 'montserrat_medium';
            font-size: 16px;
            line-height: 23px;
            color: #354049;
        }

        &__avatar {
            width: 40px;
            height: 40px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            h1 {
                font-family: 'montserrat_medium';
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