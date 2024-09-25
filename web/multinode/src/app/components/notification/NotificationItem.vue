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
import { Component, Vue, Prop } from "vue-property-decorator";
import { VAlert } from "vuetify/lib";
import { DelayedNotification } from "@/app/types/delayedNotification";

@Component({
  components: {
    VAlert,
  },
})
export default class NotificationItem extends Vue {
  @Prop({ required: true }) readonly item!: DelayedNotification;

  onMouseOver(id: string): void {
    this.$store.dispatch('notification/pause',id);
  }

  onMouseLeave(id: string): void {
    this.$store.dispatch('notification/resume',id);
  }

  onCloseClick(id: string): void {
    this.$store.dispatch('notification/delete',id);
  }
}
</script>
