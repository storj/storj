import Vue, { ComponentOptions } from 'vue';
export declare function needToProduceProvide(original: any): boolean;
interface ProvideObj {
    managed?: {
        [k: string]: any;
    };
    managedReactive?: {
        [k: string]: any;
    };
}
declare type ProvideFunc = ((this: any) => Object) & ProvideObj;
export declare function produceProvide(original: any): ProvideFunc;
/** Used for keying reactive provide/inject properties */
export declare const reactiveInjectKey = "__reactiveInject__";
export declare function inheritInjected(componentOptions: ComponentOptions<Vue>): void;
export {};
