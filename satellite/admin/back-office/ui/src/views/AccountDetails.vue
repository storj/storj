// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-container>

    <v-row>
      <v-col cols="12" md="8">
        <PageTitleComponent title="Account Details" />
        <v-chip class="mr-2 mb-2 mb-md-0 pr-4 font-weight-medium" color="default">
          <v-tooltip activator="parent" location="top">
            This account
          </v-tooltip>
          <template v-slot:prepend>
            <svg class="mr-1" width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path
                d="M12.1271 6C14.1058 6 15.7099 7.60408 15.7099 9.58281C15.7099 10.7701 15.1324 11.8226 14.2429 12.4745C16.0273 13.1299 17.4328 14.5717 18.0402 16.3804C18.059 16.4363 18.077 16.4925 18.0942 16.5491L18.1195 16.6342C18.2377 17.0429 18.0022 17.4701 17.5934 17.5883C17.5239 17.6084 17.4518 17.6186 17.3794 17.6186H6.764C6.34206 17.6186 6 17.2765 6 16.8545C6 16.7951 6.00695 16.7358 6.02069 16.678L6.02974 16.6434C6.05458 16.5571 6.08121 16.4714 6.10959 16.3866C6.7237 14.5517 8.15871 13.0936 9.97792 12.4495C9.10744 11.7961 8.54432 10.7552 8.54432 9.58281C8.54432 7.60408 10.1484 6 12.1271 6ZM12.076 13.2168C9.95096 13.2168 8.07382 14.5138 7.29168 16.4355L7.26883 16.4925H16.8831L16.8826 16.4916C16.1224 14.5593 14.2607 13.2444 12.1429 13.2173L12.076 13.2168ZM12.1271 7.12603C10.7703 7.12603 9.67035 8.22596 9.67035 9.58281C9.67035 10.9397 10.7703 12.0396 12.1271 12.0396C13.4839 12.0396 14.5839 10.9397 14.5839 9.58281C14.5839 8.22596 13.4839 7.12603 12.1271 7.12603Z"
                fill="currentColor" />
            </svg>
          </template>
          itacker@gmail.com
        </v-chip>

        <v-chip class="mr-2 mb-2 mb-md-0" variant="text">
          Customer for 17 days
          <v-tooltip activator="parent" location="top">
            Account created: 24 Apr 2020
          </v-tooltip>
        </v-chip>

      </v-col>
      <v-col cols="12" md="4" class="d-flex justify-start justify-md-end align-top align-md-center">
        <v-btn>
          <template v-slot:append>
            <v-icon icon="mdi-chevron-down"></v-icon>
          </template>
          Account Actions
          <AccountActionsMenu />
        </v-btn>
      </v-col>
    </v-row>

    <v-row>
      <v-col cols="12" sm="6" md="3">
        <v-card title="Account" subtitle="Irving Tacker" variant="flat" :border="true" rounded="xlg">
          <v-card-text>
            <v-chip color="success" variant="tonal" class="mr-2 font-weight-bold">
              Pro
              <v-tooltip activator="parent" location="top">
                Pro account since: 2 May 2022
              </v-tooltip>
            </v-chip>
            <v-divider class="my-4"></v-divider>
            <v-btn variant="outlined" size="small" color="default">
              Edit Account Information
              <AccountInformationDialog />
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>

      <v-col cols="12" sm="6" md="3">
        <v-card title="Status" subtitle="Account" variant="flat" :border="true" rounded="xlg">
          <v-card-text>
            <v-chip color="success" variant="tonal" class="mr-2 font-weight-bold">
              Active
            </v-chip>
            <v-divider class="my-4"></v-divider>
            <v-btn variant="outlined" size="small" color="default">
              Set Account Status
              <AccountStatusDialog />
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>

      <v-col cols="12" sm="6" md="3">

        <v-card title="Value" subtitle="Attribution" variant="flat" :border="true" rounded="xlg" class="mb-3">
          <v-card-text>
            <!-- <p class="mb-3">Attribution</p> -->
            <v-chip variant="tonal" class="mr-2">Company</v-chip>
            <v-divider class="my-4"></v-divider>
            <v-btn variant="outlined" size="small" color="default">
              Set Value Attribution
              <AccountUserAgentsDialog />
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>

      <v-col cols="12" sm="6" md="3">
        <v-card title="Placement" subtitle="Region" variant="flat" :border="true" rounded="xlg">
          <v-card-text>
            <!-- <p class="mb-3">Region</p> -->
            <v-chip variant="tonal" class="mr-2">
              Global
            </v-chip>
            <v-divider class="my-4"></v-divider>
            <v-btn variant="outlined" size="small" color="default">
              Set Account Placement
              <AccountGeofenceDialog />
            </v-btn>
          </v-card-text>
        </v-card>
      </v-col>

    </v-row>

    <v-row>
      <v-col cols="12" sm="6" md="3">
        <CardStatsComponent title="Projects" subtitle="Total" data="3" />
      </v-col>
      <v-col cols="12" sm="6" md="3">
        <CardStatsComponent title="Storage" subtitle="Total" data="273 TB" />
      </v-col>
      <v-col cols="12" sm="6" md="3">
        <CardStatsComponent title="Download" subtitle="This month" data="192 TB" />
      </v-col>
      <v-col cols="12" sm="6" md="3">
        <CardStatsComponent title="Segments" subtitle="Total" data="430,721" />
      </v-col>
    </v-row>

    <v-row>
      <v-col>
        <h3 class="my-4">Projects</h3>
        <AccountProjectsTableComponent />
      </v-col>
    </v-row>

    <v-row>
      <v-col>
        <h3 class="my-4">History</h3>
        <LogsTableComponent />
      </v-col>
    </v-row>

  </v-container>
</template>

<script>
import PageTitleComponent from '@/components/PageTitleComponent.vue';
import PageSubtitleComponent from '@/components/PageSubtitleComponent.vue';
import UsageProgressComponent from '@/components/UsageProgressComponent.vue';
import AccountProjectsTableComponent from '@/components/AccountProjectsTableComponent.vue';
import LogsTableComponent from '@/components/LogsTableComponent.vue';
import AccountActionsMenu from '@/components/AccountActionsMenu.vue';
import AccountLimitsDialog from '@/components/AccountLimitsDialog.vue';
import AccountUserAgentsDialog from '@/components/AccountUserAgentsDialog.vue';
import AccountGeofenceDialog from '@/components/AccountGeofenceDialog.vue';
import AccountInformationDialog from '@/components/AccountInformationDialog.vue';
import AccountStatusDialog from '@/components/AccountStatusDialog.vue';
import CardStatsComponent from '@/components/CardStatsComponent.vue';

export default {
  components: {
    PageTitleComponent,
    PageSubtitleComponent,
    UsageProgressComponent,
    AccountProjectsTableComponent,
    LogsTableComponent,
    AccountActionsMenu,
    AccountLimitsDialog,
    AccountUserAgentsDialog,
    AccountGeofenceDialog,
    AccountInformationDialog,
    AccountStatusDialog,
    CardStatsComponent
  },
  mounted() {
    document.title = "Storj Admin - Account Details";
  },
  data() {
    return {
      dialog: false,
      valid: false,
    }
  },
}
</script>
