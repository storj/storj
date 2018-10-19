import { shallowMount, mount } from '@vue/test-utils';
import Checkbox from '@/components/Checkbox.vue';

describe('Checkbox.vue', () => {
	
	it('renders correctly', () => {

    	const wrapper = shallowMount(Checkbox);

		expect(wrapper).toMatchSnapshot();
  	});
  
  	it('emit setData on change correctly', () => {

		const wrapper = mount(Checkbox);

		wrapper.find("input").trigger('change');
		wrapper.find("input").trigger('change');
		
		expect(wrapper.emitted("setData").length).toEqual(2);
	});

	it('emits with data correctly', () => {

		const wrapper = mount(Checkbox);

		wrapper.vm.$emit('setData', true);
		
		expect(wrapper.emitted("setData")[0][0]).toEqual(true);
	});
});