import Vue, { PropOptions } from 'vue';
import { Constructor } from 'vue/types/options';
/**
 * decorator of a prop
 * @param  options the options for the prop
 * @return PropertyDecorator | void
 */
export declare function Prop(options?: PropOptions | Constructor[] | Constructor): (target: Vue, key: string) => void;
