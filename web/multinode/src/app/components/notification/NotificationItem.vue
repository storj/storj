// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-alert
        border="left"
        :type="item.type.toLowerCase()"
        dismissible
        @mouseover="() => onMouseOver(item.id)"
        @mouseleave="() => onMouseLeave(item.id)"
        @input="() => onCloseClick(item.id)"
    >
        <div class="text-h6">{{ item.title }}</div>
        <div>{{ item.message }}</div>
    </v-alert>
</template>

<script lang="ts">
import { Component, Vue, Prop } from 'vue-property-decorator';
import { VAlert } from 'vuetify/lib';

import { DelayedNotification } from '@/app/types/delayedNotification';

// @vue/component
@Component({
    components: {
        VAlert,
    },
})
export default class NotificationItem extends Vue {
  @Prop({ required: true }) readonly item!: DelayedNotification;

  /**
 * Forces notification to stay on page on mouse over it.
 */
  onMouseOver(id: string): void {
      this.$store.dispatch('notification/pause', id);
  }

  /**
 * Resume notification flow when mouse leaves notification.
 */
  onMouseLeave(id: string): void {
      this.$store.dispatch('notification/resume', id);
  }

  /**
 * Removes notification when the close button is clicked.
 */
  onCloseClick(id: string): void {
      this.$store.dispatch('notification/delete', id);
  }
}
</script>
