// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- To close popup we need to use method onCloseClick -->
    <div class="api-keys-dropdown-choice-container">
        <div class="api-keys-dropdown-overflow-container">
            <!-- TODO: add selection logic onclick -->
            <div class="api-keys-dropdown-item-container" v-on:click="onSortUsersClick">
                <h2>test</h2>
            </div>
            <div class="api-keys-dropdown-item-container" v-on:click="onSortUsersClick">
                <h2>test</h2>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

@Component(
    {
        data: function () {
            return {};
        },
        props: {
            onClose: {
                type: Function
            }
        },
        methods: {
            onCloseClick: function (): void {
                this.$emit('onClose');
            },
            onSortUsersClick: function (): void {
                this.$emit('onClose');
            }
        },
    }
)

export default class Dropdown extends Vue {
}
</script>

<style scoped lang="scss">
    .api-keys-dropdown-choice-container {
        position: absolute;
        width: 100%;
        top: 70px;
        left: 0px;
        border-radius: 4px;
        padding: 10px 0px 10px 0px;
        box-shadow: 0px 4px rgba(231, 232, 238, 0.6);
        background-color: #FFFFFF;
        z-index: 800;
    }

    .api-keys-dropdown-overflow-container {
        position: relative;
        width: 100%;
        height: auto;
        background-color: #FFFFFF;
    }

    .api-keys-dropdown-item-container {
        display: flex;
        flex-direction: row;
        align-items: center;
        justify-content: flex-start;
        padding-left: 20px;
        padding-right: 20px;

        h2 {
            font-family: 'montserrat_regular';
            margin-left: 20px;
            font-size: 14px;
            line-height: 20px;
            color: #354049;
        }

        &:hover {
            background-color: #F2F2F6;

            path {
                fill: #2683FF !important;
            }
        }

    }

    input {
        outline: none;
        box-shadow: none;
    }

    a {
        text-decoration: none;
        outline: none;
    }
</style>