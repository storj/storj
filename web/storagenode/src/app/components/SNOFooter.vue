// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="footer" v-if="isShown">
        <div class="footer__content-holder">
            <StorjIcon
                class="footer__content-holder__icon"
                alt="storj icon"
                @click="scrollUp"
            />
            <div class="footer__content-holder__links-area">
                <a
                    class="footer__content-holder__links-area__community-link"
                    href="https://forum.storj.io/c/sno-category"
                    target="_blank"
                    rel="noopener"
                >
                    Community
                </a>
                <a
                    class="footer__content-holder__links-area__support-link"
                    href="https://support.storj.io"
                    target="_blank"
                    rel="noopener"
                >
                    Support
                </a>
            </div>
         </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import StorjIcon from '@/../static/images/storjIcon.svg';

import { RouteConfig } from '@/app/router';

@Component({
    components: {
        StorjIcon,
    },
})
export default class SNOFooter extends Vue {
    public scrollUp(): void {
        window.scrollTo(0, 0);
    }

    /**
     * Indicates if footer should appear.
     */
    public get isShown(): boolean {
        return this.$route.name !== RouteConfig.Notifications.name;
    }
}
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
                cursor: pointer;
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

    .storj-logo {

        path {
            fill: var(--icon-color) !important;
        }
    }

    @media screen and (max-width: 600px) {

        .footer {
            height: auto;
            padding: 25px 36px;

            &__content-holder {
                flex-direction: column;
                justify-content: flex-start;
                align-items: flex-start;

                &__links-area {
                    margin-top: 25px;
                }
            }
        }
    }
</style>
