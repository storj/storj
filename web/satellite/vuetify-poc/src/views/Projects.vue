// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container>
        <PageTitleComponent title="My Projects" />
        <!-- <PageSubtitleComponent subtitle="Projects are where you and your team can upload and manage data, view usage statistics and billing."/> -->

        <v-row>
            <v-col>
                <v-btn
                    class="mr-3"
                    color="default"
                    variant="outlined"
                    density="comfortable"
                >
                    <svg width="14" height="14" class="mr-2" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M10 1C14.9706 1 19 5.02944 19 10C19 14.9706 14.9706 19 10 19C5.02944 19 1 14.9706 1 10C1 5.02944 5.02944 1 10 1ZM10 2.65C5.94071 2.65 2.65 5.94071 2.65 10C2.65 14.0593 5.94071 17.35 10 17.35C14.0593 17.35 17.35 14.0593 17.35 10C17.35 5.94071 14.0593 2.65 10 2.65ZM10.7496 6.8989L10.7499 6.91218L10.7499 9.223H12.9926C13.4529 9.223 13.8302 9.58799 13.8456 10.048C13.8602 10.4887 13.5148 10.8579 13.0741 10.8726L13.0608 10.8729L10.7499 10.873L10.75 13.171C10.75 13.6266 10.3806 13.996 9.925 13.996C9.48048 13.996 9.11807 13.6444 9.10066 13.2042L9.1 13.171L9.09985 10.873H6.802C6.34637 10.873 5.977 10.5036 5.977 10.048C5.977 9.60348 6.32857 9.24107 6.76882 9.22366L6.802 9.223H9.09985L9.1 6.98036C9.1 6.5201 9.46499 6.14276 9.925 6.12745C10.3657 6.11279 10.7349 6.45818 10.7496 6.8989Z" fill="currentColor" />
                    </svg>
                    <!-- <IconNew class="mr-2" width="12px"/> -->
                    Create Project

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
                                    <template #prepend>
                                        <v-card-title class="font-weight-bold">
                                            <!-- <v-icon
                        >
                        <img src="../assets/icon-project.svg" alt="Project">
                        </v-icon> -->
                                            Create New Project
                                        </v-card-title>
                                    </template>

                                    <template #append>
                                        <v-btn
                                            icon="$close"
                                            variant="text"
                                            size="small"
                                            color="default"
                                            @click="dialog = false"
                                        />
                                    </template>
                                </v-card-item>
                            </v-sheet>

                            <v-divider />

                            <v-form v-model="valid" class="pa-7 pb-4">
                                <v-row>
                                    <v-col>
                                        <p>Enter project name (max 20 characters).</p>
                                    </v-col>
                                </v-row>

                                <v-row>
                                    <v-col
                                        cols="12"
                                    >
                                        <v-text-field
                                            v-model="name"
                                            variant="outlined"
                                            :rules="nameRules"
                                            label="Project Name"
                                            class="mt-2"
                                            required
                                            autofocus
                                            counter
                                            maxlength="20"
                                        />
                                    </v-col>
                                </v-row>

                                <v-row>
                                    <v-col>
                                        <v-btn variant="text" size="small" class="mb-4" color="default">+ Add Description (Optional)</v-btn>
                                    </v-col>
                                </v-row>

                                <!-- <v-row>
                      <v-col
                        cols="12"
                      >
                        <v-text-field
                          v-model="description"
                          variant="outlined"
                          label="Project Description (Optional)"
                          class="mt-2"
                          multiple
                        ></v-text-field>
                      </v-col>
                    </v-row> -->
                            </v-form>

                            <v-divider />

                            <v-card-actions class="pa-7">
                                <v-row>
                                    <v-col>
                                        <v-btn variant="outlined" color="default" block @click="dialog = false">Cancel</v-btn>
                                    </v-col>
                                    <v-col>
                                        <v-btn color="primary" variant="flat" block>Create Project</v-btn>
                                    </v-col>
                                </v-row>
                            </v-card-actions>
                        </v-card>
                    </v-dialog>
                </v-btn>
            </v-col>

            <v-spacer />

            <v-col class="text-right">
                <!-- Projects Card/Table View -->
                <v-btn-toggle
                    v-model="activeView"
                    mandatory
                    border
                    inset
                    density="comfortable"
                    class="pa-1"
                >
                    <v-btn
                        size="small"
                        rounded="xl"
                        active-class="active"
                        :active="activeView === 'cards'"
                        aria-label="Toggle Cards View"
                        @click="toggleView('cards')"
                    >
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <rect x="6.99902" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                            <rect x="6.99902" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                            <rect x="12.999" y="6.99951" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                            <rect x="12.999" y="13.0005" width="4.0003" height="4.0003" rx="1" fill="currentColor" />
                        </svg>
                        Cards
                    </v-btn>
                    <v-btn
                        size="small"
                        rounded="xl"
                        active-class="active"
                        :active="activeView === 'table'"
                        aria-label="Toggle Table View"
                        @click="toggleView('table')"
                    >
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M9 8C9 8.55228 8.55228 9 8 9V9C7.44772 9 7 8.55228 7 8V8C7 7.44772 7.44772 7 8 7V7C8.55228 7 9 7.44772 9 8V8Z" fill="currentColor" />
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M9 12C9 12.5523 8.55228 13 8 13V13C7.44772 13 7 12.5523 7 12V12C7 11.4477 7.44772 11 8 11V11C8.55228 11 9 11.4477 9 12V12Z" fill="currentColor" />
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M9 16C9 16.5523 8.55228 17 8 17V17C7.44772 17 7 16.5523 7 16V16C7 15.4477 7.44772 15 8 15V15C8.55228 15 9 15.4477 9 16V16Z" fill="currentColor" />
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M18 8C18 8.55228 17.5523 9 17 9H11C10.4477 9 10 8.55228 10 8V8C10 7.44772 10.4477 7 11 7H17C17.5523 7 18 7.44772 18 8V8Z" fill="currentColor" />
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M18 12C18 12.5523 17.5523 13 17 13H11C10.4477 13 10 12.5523 10 12V12C10 11.4477 10.4477 11 11 11H17C17.5523 11 18 11.4477 18 12V12Z" fill="currentColor" />
                            <path fill-rule="evenodd" clip-rule="evenodd" d="M18 16C18 16.5523 17.5523 17 17 17H11C10.4477 17 10 16.5523 10 16V16C10 15.4477 10.4477 15 11 15H17C17.5523 15 18 15.4477 18 16V16Z" fill="currentColor" />
                        </svg>
                        Table
                    </v-btn>
                </v-btn-toggle>
            </v-col>
        </v-row>

        <v-row v-if="activeView === 'cards'">
            <!-- Card view -->
            <v-col cols="12" sm="6" md="4" lg="3">
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-card-item>
                        <div class="d-flex justify-space-between">
                            <v-chip rounded color="purple2" variant="tonal" class="font-weight-bold my-2" size="small"><IconProject width="12px" class="mr-1" />Owner</v-chip>

                            <!-- <v-btn color="default" variant="text" size="small" icon="mdi-dots-vertical">
              </v-btn> -->

                            <v-btn color="default" variant="text" size="small">
                                <v-icon icon="mdi-dots-vertical" />

                                <v-menu activator="parent" location="end" transition="scale-transition">
                                    <!-- Project Menu -->
                                    <v-list class="pa-2">
                                        <!-- Project Settings -->
                                        <v-list-item link rounded="lg">
                                            <template #prepend>
                                                <IconSettings />
                                            </template>
                                            <v-list-item-title class="text-body-2 ml-3">
                                                Project Settings
                                            </v-list-item-title>
                                        </v-list-item>

                                        <v-divider class="my-2" />

                                        <!-- Invite Members -->
                                        <v-list-item link class="mt-1" rounded="lg">
                                            <template #prepend>
                                                <IconTeam />
                                            </template>
                                            <v-list-item-title class="text-body-2 ml-3">
                                                Invite Members
                                            </v-list-item-title>
                                        </v-list-item>
                                    </v-list>
                                </v-menu>
                            </v-btn>
                        </div>
                        <v-card-title>
                            <router-link class="link" to="/dashboard">My first project</router-link>
                        </v-card-title>
                        <v-card-subtitle>
                            <p>Project Description</p>
                        </v-card-subtitle>
                    </v-card-item>
                    <v-card-text>
                        <v-divider class="mt-1 mb-4" />
                        <v-btn color="primary" size="small" class="mr-2" link router-link to="/dashboard">Open Project</v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="4" lg="3">
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-card-item>
                        <v-chip rounded color="green" variant="tonal" class="font-weight-bold my-2" size="small"><IconProject width="12px" class="mr-1" />Member</v-chip>
                        <v-card-title>
                            <router-link class="link" to="/dashboard">Storj Labs</router-link>
                        </v-card-title>
                        <v-card-subtitle>
                            <p>Shared team project</p>
                        </v-card-subtitle>
                    </v-card-item>
                    <v-card-text>
                        <v-divider class="mt-1 mb-4" />
                        <v-btn color="primary" size="small" class="mr-2" link router-link to="/dashboard">Open Project</v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="4" lg="3">
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-card-item>
                        <v-chip rounded color="warning" variant="tonal" class="font-weight-bold my-2" size="small"><IconProject width="12px" class="mr-1" />Invited</v-chip>
                        <v-card-title>
                            Invitation Project
                        </v-card-title>
                        <v-card-subtitle>
                            <p>Example invitation.</p>
                        </v-card-subtitle>
                    </v-card-item>
                    <v-card-text>
                        <v-divider class="mt-1 mb-4" />
                        <v-btn color="primary" size="small" class="mr-2" link router-link to="/dashboard">Join Project</v-btn>
                        <v-btn variant="outlined" color="default" size="small" class="mr-2">Decline</v-btn>
                    </v-card-text>
                </v-card>
            </v-col>

            <v-col cols="12" sm="6" md="4" lg="3">
                <v-card variant="flat" :border="true" rounded="xlg">
                    <v-card-item>
                        <v-chip rounded color="primary" variant="tonal" class="font-weight-bold my-2" size="small"><IconProject width="12px" class="mr-1" />Project</v-chip>
                        <v-card-title>
                            Welcome
                        </v-card-title>
                        <v-card-subtitle>
                            <p>Create a project to get started.</p>
                        </v-card-subtitle>
                    </v-card-item>
                    <v-card-text>
                        <v-divider class="mt-1 mb-4" />
                        <v-btn color="primary" size="small" class="mr-2" link router-link to="/dashboard">Create Project</v-btn>
                    </v-card-text>
                </v-card>
            </v-col>
        </v-row>

        <v-row v-else-if="activeView === 'table'">
            <!-- Table view -->
            <v-col>
                <ProjectsTableComponent />
            </v-col>
        </v-row>

        <v-row />
    </v-container>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VContainer,
    VRow,
    VCol,
    VBtn,
    VDialog,
    VCard,
    VSheet,
    VCardItem,
    VCardTitle,
    VDivider,
    VForm,
    VTextField,
    VCardActions,
    VSpacer,
    VBtnToggle,
    VChip,
    VIcon,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VCardSubtitle,
    VCardText,
} from 'vuetify/components';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import ProjectsTableComponent from '@poc/components/ProjectsTableComponent.vue';
import IconProject from '@poc/components/icons/IconProject.vue';
import IconSettings from '@poc/components/icons/IconSettings.vue';
import IconTeam from '@poc/components/icons/IconTeam.vue';

const dialog = ref<boolean>(false);
const valid = ref<boolean>(false);
const name = ref<string>('');
const activeView = ref<string>(localStorage.getItem('activeView') || 'cards');

const nameRules = [
    value => (!!value || 'Project name is required.'),
    value => ((value?.length <= 100) || 'Name must be less than 100 characters.'),
];

function toggleView(view: string): void {
    activeView.value = view;
    localStorage.setItem('activeView', view);
}
</script>
