// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :style="configuration.style" class="notification-wrap" @mouseover="onMouseOver" @mouseleave="onMouseLeave" >
        <div class="notification-wrap__text">
            <div v-html="configuration.imageSource"></div>
            <p>{{message}}</p>
        </div>
        <div class="notification-wrap__buttons-group" v-on:click="onCloseClick">
            <span class="notification-wrap__buttons-group__close" v-html="configuration.closeImage"></span>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { NOTIFICATION_IMAGES, NOTIFICATION_TYPES } from '@/utils/constants/notification';
import { NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    props: {
        type: String,
        message: String,
    },
    methods: {
        // Force delete notification
        onCloseClick: function (): void {
            this.$store.dispatch(NOTIFICATION_ACTIONS.DELETE);
        },
        // Force notification to stay on page on mouse over it
        onMouseOver: function (): void {
            this.$store.dispatch(NOTIFICATION_ACTIONS.PAUSE);
        },
        // Resume notification flow when mouse leaves notification
        onMouseLeave: function (): void {
            this.$store.dispatch(NOTIFICATION_ACTIONS.RESUME);
        },
    },
    computed: {
        configuration: function () {
            let backgroundColor;
            let imageSource;

            // Switch for choosing notification style depends on notification type
            switch (this.$props.type) {
                case NOTIFICATION_TYPES.SUCCESS:
                    backgroundColor = 'rgba(214, 235, 208, 0.4)';
                    imageSource = NOTIFICATION_IMAGES.SUCCESS;
                    break;

                case NOTIFICATION_TYPES.ERROR:
                    backgroundColor = 'rgba(246, 205, 204, 0.4)';
                    imageSource = NOTIFICATION_IMAGES.ERROR;
                    break;
                case NOTIFICATION_TYPES.NOTIFICATION:
                default:
                    backgroundColor = 'rgba(219, 225, 232, 0.4)';
                    imageSource = NOTIFICATION_IMAGES.NOTIFICATION;
                    break;
            }

            return {
                style: {
                    backgroundColor
                },
                imageSource,
                closeImage: NOTIFICATION_IMAGES.CLOSE
            };
        },
    }
})

export default class Notification extends Vue {
}
</script>

<style scoped lang="scss">
    .notification-wrap {
        width: 100%;
        height: 98px;
        display: flex;
        justify-content: space-between;
        padding: 0 50px;
        align-items: center;

        &__text {
            display: flex;
            align-items: center;

            p {
                font-family: 'font_medium';
                font-size: 16px;
                margin-left: 40px;

                span {
                    margin-right: 10px;
                }
            }
        }

        &__buttons-group {
            display: flex;
            
            &__close {
                width: 32px;
                height: 32px;
                cursor: pointer;
            }
        }
    }
</style>
