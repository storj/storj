// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-dialog v-model="dialog" activator="parent" width="auto" transition="fade-transition">
    <v-card rounded="xlg">

      <v-sheet>
        <v-card-item class="pl-7 py-4">
          <template v-slot:prepend>
            <v-card-title class="font-weight-bold">
              Set Project Placement
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
          <v-col cols="12">
            <p>Select a placement region for this project.</p>
            <p>Applies to all the buckets and data.</p>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12">
            <v-select label="Placement region" placeholder="Select the placement region."
              :items="['global', 'us-select-1']" variant="outlined" autofocus hide-details="auto"></v-select>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12">
            <v-text-field model-value="56F82SR21Q284" label="Project ID" variant="solo-filled" flat readonly
              hide-details="auto"></v-text-field>
          </v-col>
        </v-row>

      </v-form>

      <v-divider></v-divider>

      <v-card-actions class="pa-7">
        <v-row>
          <v-col>
            <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
          </v-col>
          <v-col>
            <v-btn color="primary" variant="flat" block :loading="loading" @click="onButtonClick">Save</v-btn>
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
      text: `The project placement was set successfully.`,
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