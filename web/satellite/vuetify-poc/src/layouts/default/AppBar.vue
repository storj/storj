// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>

  <v-app-bar :elevation="0">

    <v-app-bar-nav-icon
      variant="text"
      color="default"
      class="ml-1"
      size="x-small"
      density="comfortable"
      @click.stop="drawer = !drawer"
    ></v-app-bar-nav-icon>

    <v-app-bar-title class="mx-1">
       <v-img
          v-if="theme.global.current.value.dark"
          src="@poc/assets/logo-dark.svg"
          width="120"
          alt="Storj Logo"
        />
       <v-img
          v-else
          src="@poc/assets/logo.svg"
          width="120"
          alt="Storj Logo"
        />
    </v-app-bar-title>

    <template v-slot:append>

      <v-menu offset-y width="200" class="rounded-xl">

        <template v-slot:activator="{ props }">
          <!-- Theme Toggle Light/Dark Mode -->
          <v-btn-toggle
            v-model="activeTheme"
            mandatory
            border
            inset
            density="comfortable"
            class="pa-1"
            >

            <v-tooltip text="Light Theme" location="bottom">
              <template v-slot:activator="{ props }">
                <v-btn
                  v-bind="props"
                  icon="mdi-weather-sunny"
                  size="small"
                  rounded="xl"
                  @click="toggleTheme('light')"
                  aria-label="Toggle Light Theme"
                  >
                </v-btn>
              </template>
            </v-tooltip>

            <v-tooltip text="Dark Theme" location="bottom">
              <template v-slot:activator="{ props }">
                <v-btn
                  v-bind="props"
                  icon="mdi-weather-night"
                  size="small"
                  rounded="xl"
                  @click="toggleTheme('dark')"
                  aria-label="Toggle Dark Theme"
                  >
                </v-btn>
              </template>
            </v-tooltip>

          </v-btn-toggle>

          <!-- My Account Dropdown Button -->
          <v-btn
            v-bind="props"
            variant="outlined"
            color="default"
            class="ml-4 font-weight-medium"
            density="comfortable"
          >
          <template v-slot:append>
            <img src="@poc/assets/icon-dropdown.svg" alt="Account Dropdown">
          </template>
          My Account</v-btn>
        </template>

        <!-- My Account Menu -->
        <v-list class="px-2">

          <v-list-item class="py-2 rounded-lg">
            <template v-slot:prepend>
              <img src="@poc/assets/icon-satellite.svg" alt="Region">
            </template>
            <v-list-item-title class="text-body-2 ml-3">Region</v-list-item-title>
            <v-list-item-subtitle class="ml-3">
              North America 1
            </v-list-item-subtitle>
          </v-list-item>

          <v-divider class="my-2"></v-divider>

          <v-list-item link class="my-1 rounded-lg">
            <template v-slot:prepend>
              <img src="@poc/assets/icon-upgrade.svg" alt="Upgrade">
            </template>
            <v-list-item-title class="text-body-2 ml-3">
              Upgrade
            </v-list-item-title>
          </v-list-item>

          <v-list-item link class="my-1 rounded-lg" router-link to="/billing">
            <template v-slot:prepend>
              <img src="@poc/assets/icon-card.svg" alt="Billing">
            </template>
            <v-list-item-title class="text-body-2 ml-3">
              Billing
            </v-list-item-title>
          </v-list-item>

          <v-list-item link class="my-1 rounded-lg" router-link to="/account-settings">
            <template v-slot:prepend>
              <img src="@poc/assets/icon-settings.svg" alt="Account Settings">
            </template>
            <v-list-item-title class="text-body-2 ml-3">
              Settings
            </v-list-item-title>
          </v-list-item>
          <v-list-item class="rounded-lg" link>
            <template v-slot:prepend>
              <img src="@poc/assets/icon-logout.svg" alt="Log Out">
            </template>
            <v-list-item-title class="text-body-2 ml-3">
              Sign Out
            </v-list-item-title>
          </v-list-item>

        </v-list>
      </v-menu>

    </template>

  </v-app-bar>

</template>

<script>
import { useTheme } from 'vuetify'

export default {
  setup () {
    const theme = useTheme()

    return {
      theme,
      toggleTheme: (newTheme) => {
        if ((newTheme === 'dark' && theme.global.current.value.dark) || (newTheme === 'light' && !theme.global.current.value.dark)) {
          return;
        }
        theme.global.name.value = newTheme;
        localStorage.setItem('theme', newTheme);  // Store the selected theme in localStorage
      }
    }
  },
  data: () => ({
      drawer: true,
      menu: false,
      activeTheme: null,
  }),
  watch: {
    'theme.global.current.value.dark': function(newVal) {
      this.activeTheme = newVal ? 1 : 0;
    }
  },
  created() {
    // Check for stored theme in localStorage. If none, default to 'light'
    const storedTheme = localStorage.getItem('theme') || 'light';
    this.toggleTheme(storedTheme);
    this.activeTheme = this.theme.global.current.value.dark ? 1 : 0;
  }
}
</script>


