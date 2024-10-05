// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        fullscreen
        persistent
        transition="fade-transition"
        @dragleave.self="model = false"
        @mouseout="model = false"
        @mouseleave="model = false"
    >
        <div
            class="h-100 w-100 border-white border-sm border-dashed d-flex justify-center align-center"
            @dragenter.prevent
            @dragover.prevent
            @drop.stop.prevent="(e) => emit('fileDrop', e)"
        >
            <v-alert
                rounded="lg"
                class="alert"
                color="success"
            >
                Drop your objects to put it into the “{{ bucket }}” bucket.
            </v-alert>

            <p class="info font-weight-bold text-h3 text-center">Drag and drop objects here to upload</p>
        </div>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VDialog } from 'vuetify/components';

defineProps<{
    bucket: string,
}>();

const model = defineModel<boolean>({ required: true });

const emit = defineEmits<{
    (event: 'fileDrop', value: Event): void,
}>();
</script>

<style scoped lang="scss">
.alert {
    position: absolute;
    top: 24px;
    pointer-events: none;
}

.info {
    max-width: 380px;
    color: rgb(var(--v-theme-on-primary));
}

.border-white {
    border-color: rgb(var(--v-theme-on-primary))!important;
}
</style>
