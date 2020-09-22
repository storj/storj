// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template src="./headerArea.html"></template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ProjectSelection from '@/components/header/projectsDropdown/ProjectSelection.vue';
import ResourcesSelection from '@/components/header/resourcesDropdown/ResourcesSelection.vue';
import SettingsSelection from '@/components/header/settingsDropdown/SettingsSelection.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import LogoIcon from '@/../static/images/header/logo.svg';
import NavigationCloseIcon from '@/../static/images/header/navigationClose.svg';
import NavigationMenuIcon from '@/../static/images/header/navigationMenu.svg';

import AccountButton from './accountDropdown/AccountButton.vue';

@Component({
    components: {
        AccountButton,
        NavigationArea,
        NavigationMenuIcon,
        NavigationCloseIcon,
        LogoIcon,
        ProjectSelection,
        ResourcesSelection,
        SettingsSelection,
    },
})
export default class HeaderArea extends Vue {
    /**
     * Indicates if navigation toggling button is visible depending on screen width.
     */
    public isNavigationVisible: boolean = false;

    /**
     * Toggle navigation visibility.
     */
    public toggleNavigationVisibility(): void {
        this.isNavigationVisible = !this.isNavigationVisible;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }
}
</script>

<style src="./headerArea.scss" scoped lang="scss"></style>
