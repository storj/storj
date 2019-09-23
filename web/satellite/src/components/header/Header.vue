// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./header.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import NewProjectArea from '@/components/header/NewProjectArea.vue';
import ProjectSelectionArea from '@/components/header/projectSelection/ProjectSelectionArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import ProjectCreationSuccessPopup from '@/components/project/ProjectCreationSuccessPopup.vue';

import AccountButton from './AccountButton.vue';

@Component({
    components: {
        ProjectCreationSuccessPopup,
        ProjectSelectionArea,
        NewProjectArea,
        AccountButton,
        NavigationArea,
    },
})
export default class DashboardHeader extends Vue {
    public isNavigationVisible: boolean = false;

    public toggleNavigationVisibility(): void {
        this.isNavigationVisible = !this.isNavigationVisible;
    }

    public onLogoClick(): void {
        location.reload();
    }
}
</script>

<style src="./header.scss" lang="scss"></style>
