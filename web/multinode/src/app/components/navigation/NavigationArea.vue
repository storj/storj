// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :class="['navigation-area', { '--collapsed': isCollapsed }]">
        <button type="button" class="navigation-area__toggle-btn" @click="toggleSidebar">
            <ArrowRightIcon v-if="isCollapsed" width="13" height="13" />
            <ArrowLeftIcon v-else width="13" height="13" />
        </button>
        <storj-logo class="navigation-area__logo" />
        <router-link
            v-for="navItem in navigation"
            :key="navItem.name"
            :aria-label="navItem.name"
            :class="['navigation-area__item-container', { '--collapsed': isCollapsed }]"
            :to="navItem.path"
        >
            <v-tooltip v-if="isCollapsed" location="right">
                <template #activator="{ props }">
                    <div class="navigation-area__item-container__link" v-bind="props">
                        <component :is="navItem.icon" class="navigation-icon" />
                    </div>
                </template>
                <span>{{ navItem.name }}</span>
            </v-tooltip>

            <div v-else class="navigation-area__item-container__link">
                <component :is="navItem.icon" class="navigation-icon" />
                <p :class="['navigation-area__item-container__link__title', { '--collapsed': isCollapsed }]">{{ navItem.name }}</p>
            </div>
        </router-link>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { VTooltip } from 'vuetify/components';

import { Config as RouterConfig } from '@/app/router';
import { NavigationLink } from '@/app/types/common';

import StorjLogo from '@/../static/images/Logo.svg';
import ArrowRightIcon from '@/../static/images/icons/ArrowRight.svg';
import ArrowLeftIcon from '@/../static/images/icons/ArrowLeft.svg';

const navigation: NavigationLink[] = [
    RouterConfig.MyNodes,
    RouterConfig.Wallets,
    RouterConfig.Payouts,
    RouterConfig.Bandwidth,
];

const isCollapsed = ref<boolean>(false);

function toggleSidebar(): void {
    isCollapsed.value = !isCollapsed.value;
    localStorage.setItem('collapsed', isCollapsed.value.toString());
}

onMounted(() => {
    if (localStorage.getItem('collapsed') === 'true') {
        isCollapsed.value = true;
    }
});
</script>

<style scoped lang="scss">
.navigation-area {
    box-sizing: border-box;
    padding: 30px 24px;
    height: 100vh;
    width: 280px;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    border-right: 1px solid var(--v-border-base);
    background: var(--v-background-base);
    position: relative;
    transition: width 0.3s ease, padding 0.3s ease;

    &.--collapsed {
        width: 80px;
        padding: 15px;
    }

    &__logo {
        margin-bottom: 62px;
        transition: margin 0.3s ease;
    }

    &__toggle-btn {
        position: absolute;
        top: 27px;
        right: -13px;
        padding: 4px;
        border-radius: 50%;
        background-color: var(--c-button-common);
        color: white;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 16px;
        transition: background 0.3s ease;
        z-index: 10;
        border: 3px solid var(--v-border-base);

        &:hover {
            background-color: var(--c-button-common-hover);
            color: var(--c-block-gray);
        }
    }

    &__item-container {
        flex: 0 0 auto;
        padding: 10px;
        width: calc(100% - 20px);
        display: flex;
        justify-content: flex-start;
        align-items: center;
        margin-bottom: 20px;
        text-decoration: none;
        transition: width 0.3s ease;

        &.--collapsed {
            width: calc(100% - 8px);
        }

        .navigation-icon {
            width: 20px;
            height: 20px;
            flex-shrink: 0;

            :deep(path) {
                fill: var(--v-header-base);
            }
        }

        &__link {
            display: flex;
            justify-content: flex-start;
            align-items: center;

            &__title {
                font-family: 'font_semiBold', sans-serif;
                font-size: 16px;
                line-height: 23px;
                margin: 0 0 0 15px;
                white-space: nowrap;
                color: var(--v-header-base);
                opacity: 1;
                visibility: visible;
                transition: opacity 0.1s ease, visibility 0.1s ease;

                &.--collapsed {
                    opacity: 0;
                    visibility: hidden;
                    transition-delay: 0s;
                }
            }
        }

        &.router-link-active,
        &:hover {
            background: var(--v-active-base);
            border-radius: 6px;

            .navigation-area__item-container__link__title {
                color: var(--v-primary-base);
            }

            .navigation-icon :deep(path) {
                fill: var(--v-primary-base) !important;
                opacity: 1;
            }
        }
    }
}
</style>
