// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-app-bar :elevation="0">

    <v-app-bar-nav-icon variant="text" color="default" class="mr-1" size="small" density="comfortable"
      @click.stop="drawer = !drawer"></v-app-bar-nav-icon>

    <v-app-bar-title class="mx-1">
      <router-link to="/dashboard">
        <v-img v-if="theme.global.current.value.dark" src="@/assets/logo-dark.svg" width="172" alt="Storj Logo" />
        <v-img v-else src="@/assets/logo.svg" width="172" alt="Storj Logo" />
      </router-link>
    </v-app-bar-title>

    <template v-slot:append>

      <v-menu offset-y class="rounded-xl">

        <template v-slot:activator="{ props }">
          <!-- Theme Toggle Light/Dark Mode -->
          <v-btn-toggle v-model="activeTheme" mandatory border inset rounded="lg" density="compact">

            <v-tooltip text="Light Theme" location="bottom">
              <template v-slot:activator="{ props }">
                <v-btn v-bind="props" icon="mdi-weather-sunny" size="x-small" class="px-4" @click="toggleTheme('light')"
                  aria-label="Toggle Light Theme">
                </v-btn>
              </template>
            </v-tooltip>

            <v-tooltip text="Dark Theme" location="bottom">
              <template v-slot:activator="{ props }">
                <v-btn v-bind="props" icon="mdi-weather-night" size="x-small" class="px-4" @click="toggleTheme('dark')"
                  aria-label="Toggle Dark Theme">
                </v-btn>
              </template>
            </v-tooltip>

          </v-btn-toggle>

          <!-- Account Dropdown Button -->
          <v-btn v-bind="props" variant="outlined" color="default" density="comfortable" class="ml-3 mr-1">
            <template v-slot:append>
              <v-icon icon="mdi-chevron-down"></v-icon>
            </template>
            Admin
          </v-btn>
        </template>

        <!-- My Account Menu -->
        <v-list class="px-1">

          <v-list-item rounded="lg">
            <template v-slot:prepend>
              <img src="@/assets/icon-satellite.svg" width="16" alt="Satellite">
            </template>
            <v-list-item-title class="text-body-2 ml-3">Satellite</v-list-item-title>
            <v-list-item-subtitle class="ml-3">
              North America 1
            </v-list-item-subtitle>
          </v-list-item>

          <v-divider class="mt-2 mb-1"></v-divider>

          <v-list-item rounded="lg" link router-link to="/admin-settings">
            <template v-slot:prepend>
              <img src="@/assets/icon-settings.svg" width="16" alt="Settings">
            </template>
            <v-list-item-title class="text-body-2 ml-3">Settings</v-list-item-title>
          </v-list-item>

          <v-list-item rounded="lg" link>
            <template v-slot:prepend>
              <img src="@/assets/icon-logout.svg" width="16" alt="Log Out">
            </template>
            <v-list-item-title class="text-body-2 ml-3">
              Sign Out
            </v-list-item-title>
          </v-list-item>

        </v-list>
      </v-menu>

    </template>

  </v-app-bar>

  <v-navigation-drawer v-model="drawer" color="surface">
    <v-sheet>
      <v-list class="px-2" variant="flat">

        <v-list-item link class="pa-4 rounded-lg">
          <v-menu activator="parent" location="end" transition="scale-transition">

            <v-list class="pa-2">

              <v-list-item link rounded="lg" active>
                <template v-slot:prepend>
                  <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                </template>
                <v-list-item-title class="text-body-2 font-weight-bold ml-3">
                  North America (US1)
                </v-list-item-title>
              </v-list-item>

              <v-list-item link rounded="lg">
                <!-- <template v-slot:prepend>
                    <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                  </template> -->
                <v-list-item-title class="text-body-2 ml-7">
                  Europe (EU1)
                </v-list-item-title>
              </v-list-item>

              <v-list-item link rounded="lg">
                <!-- <template v-slot:prepend>
                    <img src="@/assets/icon-check-color.svg" alt="Selected Project">
                  </template> -->
                <v-list-item-title class="text-body-2 ml-7">
                  Asia-Pacific (AP1)
                </v-list-item-title>
              </v-list-item>

              <v-divider class="my-2"></v-divider>

              <v-list-item link rounded="lg">
                <template v-slot:prepend>
                  <img src="@/assets/icon-settings.svg" alt="Satellite Settings">
                </template>
                <v-list-item-title class="text-body-2 ml-3">
                  Satellite Settings
                </v-list-item-title>
              </v-list-item>

            </v-list>
          </v-menu>
          <template v-slot:prepend>
            <img src="@/assets/icon-satellite.svg" alt="Satellite">
          </template>
          <v-list-item-title link class="text-body-2 ml-3">
            Satellite
          </v-list-item-title>
          <v-list-item-subtitle class="ml-3">
            North America US1
          </v-list-item-subtitle>
          <template v-slot:append>
            <img src="@/assets/icon-right.svg" alt="Project" width="10">
          </template>
        </v-list-item>

        <v-list-item link router-link to="/dashboard" class="my-1 py-3" rounded="lg">
          <template v-slot:prepend>
            <img src="@/assets/icon-dashboard.svg" alt="Dashboard">
          </template>
          <v-list-item-title class="text-body-2 ml-3">
            Dashboard
          </v-list-item-title>
        </v-list-item>

        <v-list-item link router-link to="/accounts" class="my-1" rounded="lg">
          <template v-slot:prepend>
            <img src="@/assets/icon-team.svg" alt="Accounts">
          </template>
          <v-list-item-title class="text-body-2 ml-3">
            Accounts
          </v-list-item-title>
        </v-list-item>

        <v-list-item link router-link to="/projects" class="my-1" rounded="lg">
          <template v-slot:prepend>
            <img src="@/assets/icon-project.svg" alt="Projects">
          </template>
          <v-list-item-title class="text-body-2 ml-3">
            Projects
          </v-list-item-title>
        </v-list-item>

      </v-list>
    </v-sheet>

  </v-navigation-drawer>
</template>

<script>
import { useTheme } from 'vuetify'

export default {
  setup() {
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
    'theme.global.current.value.dark': function (newVal) {
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