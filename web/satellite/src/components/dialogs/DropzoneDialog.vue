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
        <v-container
            fluid
            class="fill-height border-white border-sm border-dashed justify-center align-center"
            @dragenter.prevent
            @dragover.prevent
            @drop.stop.prevent="(e) => emit('fileDrop', e)"
        >
            <v-alert
                rounded="lg"
                class="alert"
                color="success"
            >
                Drop your files to put it into the “{{ bucket }}” bucket.
            </v-alert>

            <p class="info font-weight-bold text-h3 text-center">Drag and drop files here to upload</p>
        </v-container>
    </v-dialog>
</template>

<script setup lang="ts">
import { VAlert, VContainer, VDialog } from 'vuetify/components';
import { computed } from 'vue';

const props = defineProps<{
    modelValue: boolean,
    bucket: string,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
    (event: 'fileDrop', value: Event): void,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});
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
