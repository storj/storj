// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-container>

      <v-row class="my-4">
        <v-col>
          <h1>Design Library</h1>
        </v-col>
      </v-row>

      <v-row class="mt-10">
        <v-col>
          <h3>Button Variants</h3>
        </v-col>
      </v-row>

      <v-row>

        <v-col cols="auto">
          <v-btn>
            Default Variant
          </v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn variant="outlined">
            Outlined Variant
          </v-btn>
        </v-col>


        <v-col cols="auto">
          <v-btn variant="tonal">
            Tonal Variant
          </v-btn>
        </v-col>


        <v-col cols="auto">
          <v-btn variant="text">
          Text Variant
        </v-btn>
      </v-col>

      <v-col cols="auto">
        <v-btn variant="plain">
          Plain Variant
        </v-btn>
      </v-col>

      <v-col cols="auto">
        <v-btn disabled>
          Disabled
        </v-btn>
      </v-col>

    </v-row>

    <v-row class="mt-10">
        <v-col>
          <h3>Button Colors</h3>
        </v-col>
    </v-row>

    <v-row>
      <v-col cols="auto">
          <v-btn>
            Primary Color
          </v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn color="secondary">
            Secondary
          </v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn color="success">
            Success
          </v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn color="warning">
            Warning
          </v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn color="error">
            Error
          </v-btn>
        </v-col>
    </v-row>

    <v-row class="mt-10">
        <v-col>
          <h3>Button Icons</h3>
        </v-col>
      </v-row>

    <v-row>
      <v-col cols="auto">
        <v-btn icon="mdi-vuetify" size="small">
        </v-btn>
      </v-col>

      <v-col cols="auto">
        <v-btn prepend-icon="mdi-vuetify" variant="tonal">
          Prepend
        </v-btn>
      </v-col>

      <v-col cols="auto">
        <v-btn append-icon="mdi-vuetify" variant="outlined" color="secondary">
          Append Outlined Secondary
        </v-btn>
      </v-col>

      <v-col cols="auto">
        <v-btn
          prepend-icon="mdi-vuetify"
          append-icon="mdi-vuetify"
        >
          Prepend & Append
        </v-btn>
      </v-col>

      </v-row>

      <v-row class="mt-10">
        <v-col>
          <h3>Button Sizes</h3>
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="auto">
          <v-btn size="x-small" variant="tonal">Extra small Button</v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn size="small" variant="outlined">Small Button</v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn>Regular Button</v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn size="large" variant="outlined" color="warning">Large Outlined Warning</v-btn>
        </v-col>

        <v-col cols="auto">
          <v-btn size="x-large" variant="tonal" color="secondary">X-Large Button</v-btn>
        </v-col>
      </v-row>

      <v-row class="mt-10">
        <v-col>
          <h3>Button Blocks</h3>
        </v-col>
    </v-row>

      <v-row align="center" justify="center">
        <v-col>
          <v-btn block>Block Button Default</v-btn>
        </v-col>
        <v-col>
          <v-btn variant="tonal" color="secondary" block>Block Tonal Secondary</v-btn>
        </v-col>
        <v-col>
          <v-btn variant="tonal" color="error" block>Block Tonal Error</v-btn>
        </v-col>
      </v-row>

      <v-row class="mt-10">
        <v-col>
          <h3>Button Loading</h3>
        </v-col>
    </v-row>

      <v-row>
        <v-col>
          <v-card
            max-width="450"
            rounded="lg"
            title="Wanna try loading?"
            text="Example of loading button."
            :border="true"
          >

            <template v-slot:actions>
              <v-btn height="48">
                No Thanks
              </v-btn>

              <v-btn
                :loading="loading"
                class="flex-grow-1"
                height="48"
                variant="tonal"
                rounded="lg"
                @click="load"
              >
                Yes, let's go
              </v-btn>
            </template>
          </v-card>
        </v-col>
      </v-row>

      <v-row class="mt-10">
        <v-col>
          <h3>Button Group</h3>
        </v-col>
      </v-row>


      <v-row>
        <v-col>
        <v-card
          max-width="450"
          rounded="lg"
          text="How satisfied are you with developing using Vuetify?"
          title="Rate your experience"
          variant="flat"
          :border="true"
        >
          <template v-slot:append>
            <div class="me-n2">
              <v-btn
                icon="$close"
                  variant="plain"
              ></v-btn>
            </div>
          </template>

          <v-item-group
            v-model="model"
            class="d-flex justify-sm-space-between px-6 pt-2 pb-6"
          >
            <v-item
              v-for="n in 5"
              :key="n"
            >
              <template v-slot:default="{ toggle }">
                <v-btn
                  :icon="`mdi-numeric-${n}`"
                  :active="model != null && model + 1 >= n"
                  border
                  height="40"
                  variant="text"
                  width="40"
                  @click="toggle"
                ></v-btn>
              </template>
            </v-item>
          </v-item-group>
        </v-card>
        </v-col>
      </v-row>

      <v-row class="my-10"></v-row>

  </v-container>
</template>


<script>
  export default {
    data: () => ({
      loading: false,
      model: null,
    }),

    methods: {
      load () {
        this.loading = true
        setTimeout(() => (this.loading = false), 3000)
      },
    },
  }
</script>
