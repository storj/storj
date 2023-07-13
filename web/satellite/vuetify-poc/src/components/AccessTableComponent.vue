// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-card variant="flat" :border="true" class="rounded-xlg">
    <v-text-field
      v-model="search"
      label="Search"
      prepend-inner-icon="mdi-magnify"
      single-line
      hide-details
    ></v-text-field>

    <v-data-table
      v-model="selected"
      v-model:sort-by="sortBy"
      :headers="headers"
      :items="accesses"
      :search="search"
      class="elevation-1"
      show-select
      hover
    >
      <template v-slot:item.name="{ item }">
          <v-list-item class="font-weight-bold pl-0">
            {{ item.columns.name }}
        </v-list-item>
      </template>
      <template v-slot:item.status="{ item }">
        <v-chip :color="getColor(item.raw.status)" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
          {{ item.raw.status }}
        </v-chip>
      </template>
    </v-data-table>
  </v-card>
</template>

<script>
export default {
  name: 'AccessTableComponent',
  data () {
      return {
        search: '',
        selected: [],
        sortBy: [{ key: 'date', order: 'asc' }],
        headers: [
          {
            title: 'Name',
            align: 'start',
            key: 'name',
          },
          { title: 'Type', key: 'type' },
          { title: 'Status', key: 'status' },
          { title: 'Permissions', key: 'permissions' },
          { title: 'Date Created', key: 'date' },
        ],
        accesses: [
          {
            name: 'Backup',
            date: '02 Mar 2023',
            type: 'Access Grant',
            permissions: 'All',
            status: 'Active',
          },
          {
            name: 'S3 Test',
            date: '03 Mar 2023',
            type: 'S3 Credentials',
            permissions: 'Read, Write',
            status: 'Expired',
          },
          {
            name: 'CLI Demo',
            date: '04 Mar 2023',
            type: 'CLI Access',
            permissions: 'Read, Write, List',
            status: 'Active',
          },
          {
            name: 'Sharing',
            date: '08 Mar 2023',
            type: 'Access Grant',
            permissions: 'Read, Delete',
            status: 'Active',
          },
          {
            name: 'Sync Int',
            date: '12 Mar 2023',
            type: 'S3 Credentials',
            permissions: 'All',
            status: 'Expired',
          },
        ],
      }
    },
    computed: {
      getColor() {
        return (role) => {
          if (role === 'Owner') return 'purple2'
          if (role === 'Invited') return 'warning'
          return 'green'
        }
      }
    },
  }
</script>