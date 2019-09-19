// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import VueRouter from 'vue-router';

import TabNavigation from '@/components/navigation/TabNavigation.vue';

import { NavigationLink } from '@/types/navigation';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

localVue.use(VueRouter);

const navigation: NavigationLink[] = [
    new NavigationLink('path1', 'name1'),
    new NavigationLink('path2', 'name2'),
    new NavigationLink('path3', 'name3'),
];

describe('TabNavigation', () => {
    it('snapshot not changed', () => {
        const wrapper = shallowMount(TabNavigation, {
            localVue,
            propsData: {
                navigation,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('navigation links renders correctly', () => {
        const wrapper = shallowMount(TabNavigation, {
            localVue,
            propsData: {
                navigation,
            },
        });

        const navigationLinks = wrapper.findAll('.tab-navigation-container__item');

        expect(navigationLinks.length).toBe(navigation.length);

        for (let i = 0; i < navigation.length; i++) {
            expect(navigationLinks.at(i).text()).toBe(navigation[i].name);
        }
    });
});
