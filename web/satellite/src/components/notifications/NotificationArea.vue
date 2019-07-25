// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="notificationArea" class="notification-container" v-if="currentNotification" >
        <Notification :notification="currentNotification" />
    </div>
</template>

<script lang="ts">
    import { Component, Vue } from 'vue-property-decorator';
    import Notification from '@/components/notifications/Notification.vue';

    @Component({
        components: {
            Notification,
        }
    })
    export default class NotificationArea extends Vue {
        public get currentNotification(): Notification {
            return this.$store.getters.currentNotification;
        }
    }
</script>

<style scoped lang="scss">
    .notification-container {
        height: 98px;
        max-width: 80%;
        width: 100%;
        background-color: #fff;
        display: flex;
        position: fixed;
        bottom: 50px;
        right: 50%;
        transform: translate(50%);
        align-items: center;
        justify-content: space-between;
        box-shadow: 0px 12px 24px rgba(175, 183, 193, 0.4);
        border-radius: 6px;
        z-index: 9999;
    }
</style>
