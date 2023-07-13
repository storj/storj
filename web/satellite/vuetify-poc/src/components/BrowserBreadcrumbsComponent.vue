// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-breadcrumbs :items="['Buckets', 'Demo']" active-class="font-weight-bold" class="pa-0">
    <template v-slot:divider>
      <img src="@poc/assets/icon-right.svg" alt="Breadcrumbs separator" width="10"/>
    </template>
  </v-breadcrumbs>
</template>

<!-- <script>
export default {
  props: ['title']
}
</script> -->