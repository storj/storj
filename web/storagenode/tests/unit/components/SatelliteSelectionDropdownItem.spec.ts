// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import VueClipboard from 'vue-clipboard2';

import SatelliteSelectionDropdownItem from '@/app/components/SatelliteSelectionDropdownItem.vue';

import { SatelliteInfo } from '@/storagenode/sno/sno';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(VueClipboard);

describe('SatelliteSelectionDropdownItem', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(SatelliteSelectionDropdownItem, {
            propsData: {
                satellite: new SatelliteInfo(
                    '1',
                    'name',
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if disqualified', (): void => {
        const wrapper = mount(SatelliteSelectionDropdownItem, {
            propsData: {
                satellite: new SatelliteInfo(
                    '1',
                    'name',
                    new Date(),
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if suspended', (): void => {
        const wrapper = mount(SatelliteSelectionDropdownItem, {
            propsData: {
                satellite: new SatelliteInfo(
                    '1',
                    'name',
                    null,
                    new Date(),
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if suspended', async (): Promise<void> => {
        const satellite = new SatelliteInfo(
            '1',
            'name',
        );

        const wrapper = shallowMount(SatelliteSelectionDropdownItem, {
            localVue,
            propsData: {
                satellite,
            },
        });

        await wrapper.find('.satellite-choice__right-area__button').trigger('click');

        expect(wrapper.find('.satellite-choice__name').text()).toBe(satellite.id);
        expect(wrapper.findAll('.satellite-choice__right-area__button').length).toBe(2);

        await wrapper.findAll('.satellite-choice__right-area__button').at(1).trigger('click');

        expect(wrapper.find('.satellite-choice__name').text()).toBe(satellite.url);
    });
});
