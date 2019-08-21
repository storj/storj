// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div id="notificationArea" class="notification-container" v-if="notifications.length > 0" >
        <Notification v-for="notification in notifications" :notification="notification" />
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
        public get notifications(): Notification {
            return this.$store.state.notificationsModule.notificationQueue;
        }
    }
</script>

<style scoped lang="scss">
    .notification-container {
        width: 417px;
        background-color: transparent;
        display: flex;
        flex-direction: column;
        position: fixed;
        top: 114px;
        right: 17px;
        align-items: flex-end;
        justify-content: space-between;
        border-radius: 12px;
        z-index: 9999;
        overflow: hidden;
    }
</style>
