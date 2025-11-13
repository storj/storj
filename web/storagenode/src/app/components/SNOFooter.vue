// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isShown" class="footer">
        <div class="footer__content-holder">
            <StorjIconDark
                v-if="isDarkMode"
                class="footer__content-holder__icon"
                alt="storj icon"
            />
            <StorjIconLight
                v-else
                class="footer__content-holder__icon"
                alt="storj icon"
            />
            <div class="footer__content-holder__links-area">
                <a
                    class="footer__content-holder__links-area__community-link"
                    href="https://forum.storj.io/c/sno-category"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Community
                </a>
                <a
                    class="footer__content-holder__links-area__support-link"
                    href="https://support.storj.io"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Support
                </a>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';

import { RouteConfig } from '@/app/router';
import { useAppStore } from '@/app/store/modules/appStore';

import StorjIconLight from '@/../static/images/storjIcon.svg';
import StorjIconDark from '@/../static/images/storjIconDark.svg';

const route = useRoute();

const appStore = useAppStore();

const isShown = computed<boolean>(() => {
    return route.name !== RouteConfig.Notifications.name;
});

const isDarkMode = computed<boolean>(() => {
    return appStore.state.isDarkMode;
});
</script>

<style scoped lang="scss">
    .footer {
        padding: 0 36px;
        width: calc(100% - 72px);
        min-height: 89px;
        display: flex;
        justify-content: center;
        background-color: var(--block-background-color);
        align-items: center;

        &__content-holder {
            width: 822px;
            display: flex;
            justify-content: space-between;
            align-items: center;

            &__icon {
                min-width: 125px;
            }

            &__links-area {
                display: flex;
                flex-direction: row;
                align-items: center;
                justify-content: flex-end;

                &__community-link,
                &__support-link {
                    font-size: 14px;
                    text-decoration: none;
                    color: var(--link-color);
                }

                &__community-link {
                    margin-right: 44px;
                }
            }
        }
    }

    .storj-logo :deep(path) {
        fill: var(--icon-color) !important;
    }

    @media screen and (width <= 600px) {

        .footer {
            height: auto;
            padding: 10px 36px;
            min-height: 29px;

            &__content-holder {
                flex-direction: column;
                justify-content: flex-start;
                align-items: flex-start;

                &__icon {
                    display: none;
                }
            }
        }
    }
</style>
