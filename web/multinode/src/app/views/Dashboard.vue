// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <div class="dashboard-area">
      <div class="dashboard-area__navigation-area">
          <navigation-area />
      </div>
  </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NavigationArea from '@/app/components/navigation/NavigationArea.vue';
@Component({
    components: {NavigationArea},
})
export default class Dashboard extends Vue {}
</script>

<style lang="scss" scoped>
    .dashboard-area {
        display: flex;

        &__navigation-area {
            width: 280px;
        }
    }
</style>
