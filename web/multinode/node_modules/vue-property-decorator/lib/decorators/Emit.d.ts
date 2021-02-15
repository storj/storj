import Vue from 'vue';
/**
 * decorator of an event-emitter function
 * @param  event The name of the event
 * @return MethodDecorator
 */
export declare function Emit(event?: string): (_target: Vue, propertyKey: string, descriptor: any) => void;
