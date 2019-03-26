// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dropdown-container" >
        <div class="dropdown-toggle-container" v-on:click="toggleSelection" >
            <h1 class="dropdown-toggle-container__dropdown-name">Read / Write</h1>
            <div class="dropdown-toggle-container__expander-area">
                <img v-if="!isChoiceShown" src="../../../static/images/register/BlueExpand.svg" />
                <img v-if="isChoiceShown" src="../../../static/images/register/BlueHide.svg" />
            </div>
        </div>
        <Dropdown v-if="isChoiceShown" @onClose="toggleSelection" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Dropdown from './Dropdown.vue';

@Component(
    {
        data: function () {
            return {
                isChoiceShown: false
            };
        },
        methods: {
            toggleSelection: function (): void {
                this.$data.isChoiceShown = !this.$data.isChoiceShown;
            }
        },
        components: {
            Dropdown
        }
    }
)

export default class ApiKeysDropdown extends Vue {
}
</script>

<style scoped lang="scss">
    a {
        text-decoration: none;
        outline: none;
    }
    .dropdown-container {
        position: relative;
        background-color: #FFFFFF;
        cursor: pointer;
        font-family: 'font_regular';
        font-size: 16px;
        resize: none;
        height: 50px;
        width: 100%;
        border: 1px solid rgba(56, 75, 101, 0.4);
        border-radius: 6px;
    }

    .dropdown-toggle-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        height: 50px;

        &__dropdown-name {
            margin-left: 20px;
            font-family: 'font_medium';
            font-size: 16px;
            color: #384B65;
            opacity: 0.4;
        }

        &__expander-area {
            margin-left: 12px;
            margin-right: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 28px;
            height: 28px;
        }
    }

    @media screen and (max-width: 720px) {

        .dropdown-toggle-container {
            &__dropdown-name {
                display: none;
            }
        }
    }
</style>