// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="notifications-container">
        <div class="notifications-container__header">
            <router-link :to="'/'" class="notifications-container__header__back-link">
                <BackArrowIcon />
            </router-link>
            <p class="notifications-container__header__text">Notifications</p>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BackArrowIcon from '@/../static/images/notifications/backArrow.svg';

@Component ({
    components: {
        BackArrowIcon,
    },
})
export default class NotificationsArea extends Vue {

}
</script>

<style scoped lang="scss">
    .notifications-container {
        width: 822px;
        height: calc(100vh - 89px - 89px);

        &__header {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            margin-top: 17px;

            &__back-link {
                width: 25px;
                height: 25px;
                display: flex;
                align-items: center;
                justify-content: center;
            }

            &__text {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 57px;
                color: #535f77;
                margin-left: 29px;
                text-align: center;
            }
        }
    }
</style>
