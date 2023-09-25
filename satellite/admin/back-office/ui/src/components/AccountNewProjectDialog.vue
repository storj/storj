// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
    <v-card rounded="xlg">

      <v-sheet>
        <v-card-item class="pl-7 py-4">
          <template v-slot:prepend>
            <v-card-title class="font-weight-bold">
              Create New Project
            </v-card-title>
          </template>

          <template v-slot:append>
            <v-btn icon="$close" variant="text" size="small" color="default" @click="dialog = false"></v-btn>
          </template>
        </v-card-item>
      </v-sheet>

      <v-divider></v-divider>

      <v-form v-model="valid" class="pa-7">
        <v-row>
          <v-col>
            <p class="pb-2">Create a new project for this account.</p>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12">
            <v-text-field label="Project Name" placeholder="Enter Project Name" variant="outlined" autofocus
              hide-details="auto"></v-text-field>
          </v-col>
          <v-col cols="12">
            <v-text-field model-value="41" label="Account ID" variant="solo-filled" flat hide-details="auto"
              readonly></v-text-field>
          </v-col>
        </v-row>
      </v-form>

      <v-divider></v-divider>

      <v-card-actions class="pa-7">
        <v-row>
          <v-col>
            <v-btn size="large" variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
          </v-col>
          <v-col>
            <v-btn size="large" color="primary" variant="flat" block @click="onButtonClick">Create Project</v-btn>
          </v-col>
        </v-row>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <v-snackbar :timeout="7000" v-model="snackbar" color="success">
    {{ text }}
    <template v-slot:actions>
      <v-btn color="default" variant="text" @click="snackbar = false">
        Close
      </v-btn>
    </template>
  </v-snackbar>
</template>
  
<script>
export default {
  data() {
    return {
      snackbar: false,
      text: `Project created successfully.`,
      dialog: false,
    }
  },
  methods: {
    onButtonClick() {
      this.snackbar = true;
      this.dialog = false;
    }
  }
}
</script>