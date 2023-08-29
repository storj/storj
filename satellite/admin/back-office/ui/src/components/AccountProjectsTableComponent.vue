// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-card variant="flat" :border="true" rounded="xlg">

    <v-text-field v-model="search" label="Search" prepend-inner-icon="mdi-magnify" single-line variant="solo-filled" flat
      hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2"></v-text-field>

    <v-data-table v-model="selected" v-model:sort-by="sortBy" :headers="headers" :items="files" :search="search"
      class="elevation-1" @item-click="handleItemClick" item-key="path" density="comfortable" hover>

      <template v-slot:item.projectid="{ item }">
        <div class="text-no-wrap">
          <v-btn variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
            width="24" height="24">
            <ProjectActionsMenu />
            <v-icon icon="mdi-dots-horizontal"></v-icon>
          </v-btn>
          <v-chip variant="text" color="default" size="small" router-link to="/project-details"
            class="font-weight-bold pl-1 ml-1">
            <template v-slot:prepend>
              <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                <rect x="0.5" y="0.5" width="31" height="31" rx="10" stroke="currentColor" stroke-opacity="0.2" />
                <path
                  d="M16.2231 7.08668L16.2547 7.10399L23.4149 11.2391C23.6543 11.3774 23.7829 11.6116 23.8006 11.8529L23.8021 11.8809L23.8027 11.9121V20.1078C23.8027 20.3739 23.6664 20.6205 23.4432 20.7624L23.4136 20.7803L16.2533 24.8968C16.0234 25.029 15.7426 25.0342 15.5088 24.9125L15.4772 24.8951L8.38642 20.7787C8.15725 20.6457 8.01254 20.4054 8.00088 20.1422L8 20.1078L8.00026 11.8975L8 11.8738C8.00141 11.6177 8.12975 11.3687 8.35943 11.2228L8.38748 11.2058L15.4783 7.10425C15.697 6.97771 15.9622 6.96636 16.1893 7.07023L16.2231 7.08668ZM22.251 13.2549L16.6424 16.4939V22.8832L22.251 19.6588V13.2549ZM9.55175 13.2614V19.6611L15.0908 22.8766V16.4916L9.55175 13.2614ZM15.8669 8.67182L10.2916 11.8967L15.8686 15.149L21.4755 11.9109L15.8669 8.67182Z"
                  fill="currentColor" />
              </svg>
            </template>
            {{ item.columns.projectid }}
          </v-chip>
        </div>
      </template>

      <template v-slot:item.storagepercent="{ item }">
        <v-chip variant="tonal" :color="getPercentColor(item.raw.storagepercent)" size="small" rounded="lg"
          class="font-weight-bold">
          {{ item.raw.storagepercent }}&percnt;
        </v-chip>
      </template>

      <template v-slot:item.downloadpercent="{ item }">
        <v-chip variant="tonal" :color="getPercentColor(item.raw.downloadpercent)" size="small" rounded="lg"
          class="font-weight-bold">
          {{ item.raw.downloadpercent }}&percnt;
        </v-chip>
      </template>

      <template v-slot:item.segmentpercent="{ item }">

        <v-tooltip text="430,000 / 1,000,000">
          <template v-slot:activator="{ props }">
            <v-chip v-bind="props" variant="tonal" :color="getPercentColor(item.raw.segmentpercent)" size="small"
              rounded="lg" class="font-weight-bold">
              {{ item.raw.segmentpercent }}&percnt;
            </v-chip>
          </template>
        </v-tooltip>
      </template>

      <template v-slot:item.agent="{ item }">
        <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="setSearch(item.raw.agent)">
          {{ item.raw.agent }}
        </v-chip>
      </template>

      <template v-slot:item.date="{ item }">
        <span class="text-no-wrap">
          {{ item.raw.date }}
        </span>
      </template>

    </v-data-table>

  </v-card>
</template>

<script>
import ProjectActionsMenu from '@/components/ProjectActionsMenu.vue';

export default {
  components: {
    ProjectActionsMenu,
  },
  data() {
    return {
      // search in the table
      search: '',
      selected: [],
      sortBy: [{ key: 'name', order: 'asc' }],
      headers: [
        { title: 'Project ID', key: 'projectid', align: 'start' },
        // { title: 'Name', key: 'name'},
        { title: 'Storage Used', key: 'storagepercent' },
        { title: 'Storage Used', key: 'storageused' },
        { title: 'Storage Limit', key: 'storagelimit' },
        { title: 'Download Used', key: 'downloadpercent' },
        { title: 'Download Used', key: 'downloadused' },
        { title: 'Download Limit', key: 'downloadlimit' },
        { title: 'Segments Used', key: 'segmentpercent' },
        // { title: 'Value Attribution', key: 'agent' },
        // { title: 'Date Created', key: 'date' },
      ],
      files: [
        {
          name: 'My First Project',
          projectid: 'F82SR21Q284JF',
          storageused: '150 TB',
          storagelimit: '300 TB',
          storagepercent: '50',
          downloadused: '100 TB',
          downloadlimit: '100 TB',
          downloadpercent: '100',
          segmentpercent: '43',
          agent: 'Test Agent',
          date: '02 Mar 2023',
        },
        {
          name: 'Personal Project',
          projectid: '284JFF82SR21Q',
          storageused: '24 TB',
          storagelimit: '30 TB',
          storagepercent: '80',
          downloadused: '7 TB',
          downloadlimit: '100 TB',
          segmentpercent: '20',
          downloadpercent: '7',
          agent: 'Agent',
          date: '21 Apr 2023',
        },
        {
          name: 'Test Project',
          projectid: '82SR21Q284JFF',
          storageused: '99 TB',
          storagelimit: '100 TB',
          storagepercent: '99',
          downloadused: '85 TB',
          downloadlimit: '100 TB',
          segmentpercent: '83',
          downloadpercent: '85',
          agent: 'Company',
          date: '21 Apr 2023',
        },
      ],
    };
  },
  methods: {
    setSearch(searchText) {
      this.search = searchText
    },
    getPercentColor(percent) {
      if (percent >= 99) {
        return 'error'
      } else if (percent >= 80) {
        return 'warning'
      } else {
        return 'success'
      }
    },
  },
};
</script>
