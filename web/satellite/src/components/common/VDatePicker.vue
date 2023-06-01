// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VueDatePicker
        aria-roledescription="datepicker"
        :open="true"
        :value="date"
        :editable="false"
        :clearable="false"
        :append-to-body="false"
        popup-class="picker"
        :popup-style="{position: 'relative', left: 0, top: 0}"
        @change="onDatePick"
    />
</template>

<script setup lang="ts">
import VueDatePicker from 'vue-datepicker-next';

const props = withDefaults(defineProps<{
    onDatePick?: (date: Date) => void;
    date?: Date;
}>(), {
    onDatePick: () => () => false,
    date: () => new Date(),
});
</script>
