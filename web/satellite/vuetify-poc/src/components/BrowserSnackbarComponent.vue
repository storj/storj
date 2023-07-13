// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-snackbar
    :model-value="snackbar"
    vertical
    :timeout="4000"
    color="default"
    elevation="24"
    rounded="lg"
    class="upload-snackbar"
  >
    <v-row>
      <v-col>
        <v-expansion-panels theme="dark">
          <v-expansion-panel
            color="default"
            rounded="lg"
          >
            <v-expansion-panel-title color="">
              <span>Uploading 3 items</span>
            </v-expansion-panel-title>
            <v-progress-linear
                rounded
                model-value="73"
                height="6"
                color="success"
                class="mt-1"
            >
            </v-progress-linear>
            <v-expansion-panel-text>
              <v-row class="pt-2">
                <v-col cols="10">
                  <p class="text-medium-emphasis">4 seconds left...</p>
                </v-col>
                <v-col cols="2">

                  <v-tooltip text="Cancel all uploads">
                    <template v-slot:activator="{ props }">
                      <v-icon
                        v-bind="props"
                        icon="mdi-close-circle"
                        @click="snackbar = false"
                      ></v-icon>
                    </template>
                  </v-tooltip>

                </v-col>
                </v-row>
              </v-expansion-panel-text>
              <v-divider></v-divider>
              <v-expansion-panel-text>
                <v-row class="pt-2">
                  <v-col cols="10">
                  <p>Image.jpg</p>
                  </v-col>
                  <v-col cols="2">

                  <v-tooltip text="Uploading...">
                    <template v-slot:activator="{ props }">
                      <v-progress-circular
                      v-bind="props"
                        :size="20"
                        color="secondary"
                        model-value="20"
                      ></v-progress-circular>
                    </template>
                  </v-tooltip>

                </v-col>
              </v-row>
            </v-expansion-panel-text>
            <v-divider></v-divider>
            <v-expansion-panel-text>
              <v-row class="pt-2">
                <v-col cols="10">
                  <p>Video.mp4</p>
                  </v-col>
                  <v-col cols="2">

                  <v-tooltip text="Upload complete">
                    <template v-slot:activator="{ props }">
                      <v-icon
                      v-bind="props"
                      icon="mdi-checkbox-marked-circle"
                      color="success"
                    ></v-icon>
                    </template>
                  </v-tooltip>

                </v-col>
              </v-row>
            </v-expansion-panel-text>
            <v-divider></v-divider>
              <v-expansion-panel-text>
                <v-row class="pt-2">
                  <v-col cols="10">
                    <p>Text.pdf</p>
                    </v-col>
                    <v-col cols="2">

                    <v-tooltip text="Upload failed">
                      <template v-slot:activator="{ props }">
                        <v-icon
                          v-bind="props"
                          icon="mdi-information"
                          color="warning"
                        ></v-icon>
                      </template>
                    </v-tooltip>

                  </v-col>
                </v-row>
              </v-expansion-panel-text>
            <v-divider></v-divider>
            <v-expansion-panel-text>
              <v-row class="pt-2">
                <v-col cols="10">
                  <p>Bigvideo.mov</p>
                  </v-col>
                  <v-col cols="2">

                  <v-tooltip text="File is too big">
                    <template v-slot:activator="{ props }">
                      <v-icon
                        v-bind="props"
                        icon="mdi-cancel"
                        color="error2"
                    ></v-icon>
                    </template>
                  </v-tooltip>

                </v-col>
              </v-row>
            </v-expansion-panel-text>
            <v-divider></v-divider>
          </v-expansion-panel>

        </v-expansion-panels>
      </v-col>
    </v-row>
  </v-snackbar>
</template>

<script>
export default {
  props: {
    snackbar: {
      required: true
    }
  },
};
</script>