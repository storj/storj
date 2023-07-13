// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
  <v-dialog
    v-model="dialog"
    activator="parent"
    width="auto"
    min-width="400px"
    transition="fade-transition"
  >
    <v-card rounded="xlg">

      <v-sheet>
        <v-card-item class="pl-7 py-4">
          <template v-slot:prepend>
            <v-card-title class="font-weight-bold">
              New Folder
            </v-card-title>
          </template>

          <template v-slot:append>
            <v-btn
              icon="$close"
              variant="text"
              size="small"
              color="default"
              @click="dialog = false"
            ></v-btn>
          </template>
        </v-card-item>
      </v-sheet>

      <v-divider></v-divider>

      <v-form v-model="valid" class="pa-8 pb-3">
        <v-row>
            <v-col
              cols="12"
            >
              <v-text-field
                v-model="folder"
                variant="outlined"
                :rules="folderRules"
                label="Enter Folder Name"
                required
                autofocus
              >
              </v-text-field>
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
            <v-btn color="primary" variant="flat" block :loading="loading">Create Folder</v-btn>
          </v-col>
        </v-row>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script>
export default {
  data() {
    return {
      dialog: false,
      folder: '',
      folderRules: [
        value => {
          if (value) return true
          return 'Folder name is required.'
        },
      ],
    }
  }
}
</script>