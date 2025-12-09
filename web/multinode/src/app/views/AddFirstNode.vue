// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="add-first-node">
        <div class="add-first-node__left-area">
            <svg class="logo" width="70" height="70" viewBox="0 0 46 29" xmlns="http://www.w3.org/2000/svg">
                <g id="Page-1" stroke="none" stroke-width="1" fill="none" fill-rule="evenodd">
                    <g id="storj-logo-mark-color" fill="#788391" fill-rule="nonzero">
                        <path id="Path" d="M22.752,0 C35.3176,0 45.504,10.1864 45.504,22.752 C45.504,24.8855 45.2103,26.9504 44.6612,28.9086 L40.3217,28.9086 C40.9969,26.9814 41.364,24.9096 41.364,22.752 C41.364,12.4729 33.0311,4.13997 22.752,4.13997 C12.4729,4.13997 4.13997,12.4729 4.13997,22.752 C4.13997,23.4578 4.17926,24.1545 4.25578,24.8399 L9.67237,24.8399 L30.0505,24.8398 C31.7456,24.8398 33.1198,23.4657 33.1198,21.7705 C33.1198,20.0754 31.7456,18.7012 30.0515,18.7012 L30.0097,18.7013 L28.219,18.7021 L27.9605,16.9301 C27.5849,14.3557 25.3645,12.4199 22.7341,12.4199 C20.3731,12.4199 18.3173,13.9833 17.6641,16.2137 L17.2283,17.7019 L15.6776,17.7019 L15.2037,17.7019 C13.6268,17.7019 12.3485,18.9802 12.3485,20.5571 C12.3485,20.6049 12.3497,20.6526 12.3521,20.6999 L8.21,20.6999 C8.20965,20.6824 8.20936,20.6649 8.20914,20.6473 L8.20856,20.5571 C8.20856,17.0415 10.802,14.1316 14.1801,13.6363 L14.2356,13.6286 L14.2683,13.5609 C15.8018,10.426 18.9956,8.32858 22.608,8.28077 L22.7341,8.27994 C26.8641,8.27994 30.4251,10.9525 31.6778,14.7296 L31.6834,14.7471 L31.7829,14.7708 C34.8907,15.5374 37.2047,18.3197 37.2588,21.6513 L37.2597,21.7705 C37.2597,25.407 34.5673,28.4146 31.0672,28.9086 L0.842748,28.9086 C0.293654,26.9504 0,24.8855 0,22.752 C0,10.1864 10.1864,0 22.752,0 Z" />
                    </g>
                </g>
            </svg>
            <h1 class="add-first-node__left-area__title">Let's add your first node.</h1>
            <p class="add-first-node__left-area__info">Please add authentication data below:</p>
            <headered-input
                class="add-first-node__left-area__input"
                label="Node ID"
                placeholder="Enter Node ID"
                :error="idError"
                @set-data="setNodeId"
            />
            <headered-input
                class="add-first-node__left-area__input"
                label="Node Name"
                placeholder="Enter Node Name"
                :error="nameError"
                @set-data="setNodeName"
            />
            <headered-input
                class="add-first-node__left-area__input"
                label="Public IP Address"
                placeholder="Enter Public IP Address and Port"
                :error="publicIPError"
                @set-data="setPublicIP"
            />
            <headered-input
                class="add-first-node__left-area__input"
                label="API Key"
                placeholder="Enter API Key"
                :error="apiKeyError"
                @set-data="setApiKey"
            />
            <v-button class="add-first-node__left-area__button" label="Add Node" width="120px" :on-press="onCreate" />
        </div>
        <div class="add-first-node__right-area">
            <theme-selector class="add-first-node__right-area__theme-selector" />
            <img src="@/../static/images/Illustration.png" alt="Storj Logo Illustration">
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { Config as RouterConfig } from '@/app/router';
import { CreateNodeFields } from '@/nodes';
import { useNodesStore } from '@/app/store/nodesStore';

import HeaderedInput from '@/app/components/common/HeaderedInput.vue';
import VButton from '@/app/components/common/VButton.vue';
import ThemeSelector from '@/app/components/common/ThemeSelector.vue';

const router = useRouter();

const nodesStore = useNodesStore();

const nodeToAdd = ref<CreateNodeFields>(new CreateNodeFields());
const isLoading = ref<boolean>(false);
const idError = ref<string>('');
const publicIPError = ref<string>('');
const apiKeyError = ref<string>('');
const nameError = ref<string>('');

function setNodeId(value: string): void {
    nodeToAdd.value.id = value.trim();
    idError.value = '';
}

function setPublicIP(value: string): void {
    nodeToAdd.value.publicAddress = value.trim();
    publicIPError.value = '';
}

function setApiKey(value: string): void {
    nodeToAdd.value.apiSecret = value.trim();
    apiKeyError.value = '';
}

function setNodeName(value: string): void {
    nodeToAdd.value.name = value.trim();
    nameError.value = '';
}

function validateFields(): boolean {
    let hasNoErrors = true;

    if (!nodeToAdd.value.id) {
        idError.value = 'This field is required. Please enter a valid node ID';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.name) {
        nameError.value = 'This field is required. Please enter a valid node Name';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.publicAddress) {
        publicIPError.value = 'This field is required. Please enter a valid node Public Address';
        hasNoErrors = false;
    }

    if (!nodeToAdd.value.apiSecret) {
        apiKeyError.value = 'This field is required. Please enter a valid API Key';
        hasNoErrors = false;
    }

    return hasNoErrors;
}

async function onCreate(): Promise<void> {
    if (isLoading.value) return;

    isLoading.value = true;

    if (!validateFields()) {
        isLoading.value = false;

        return;
    }

    try {
        await nodesStore.add(nodeToAdd.value);
    } catch (error) {
        console.error(error);
        isLoading.value = false;
    }

    await router.push(RouterConfig.MyNodes.path);
}
</script>

<style lang="scss">
    .add-first-node {
        display: flex;
        box-sizing: border-box;
        height: 100%;
        background: var(--v-background-base);

        &__left-area,
        &__right-area {
            position: relative;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            justify-content: center;
            width: 50%;
            height: 100%;
        }

        &__left-area {
            padding: 0 90px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 48px;
                line-height: 60px;
                color: var(--v-header-base);
                width: 420px;
            }

            &__info {
                font-family: 'font_regular', sans-serif;
                margin-top: 16px;
                font-size: 16px;
                line-height: 29px;
                color: var(--v-text-base);
                width: 420px;
            }

            &__input {
                width: 420px;
            }

            &__button {
                margin-top: 24px;
            }
        }

        &__right-area {
            background: var(--v-background2-base);
            align-items: center;

            &__theme-selector {
                position: absolute;
                top: 20px;
                right: 20px;
            }
        }
    }

    .logo {
        position: absolute;
        left: 90px;
        top: 70px;
    }
</style>
