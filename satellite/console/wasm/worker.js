importScripts('wasm_exec.js');

if (!WebAssembly.instantiate) {
    self.postMessage(new Error('web assembly is not supported'));
}

const go = new Go();

const instantiateStreaming = WebAssembly.instantiateStreaming || async function (resp, importObject) {
    const response = await resp;
    const source = await response.arrayBuffer();

    return await WebAssembly.instantiate(source, importObject);
};

const response = fetch('access.wasm');

const methodsRun = instantiateStreaming(response, go.importObject);

methodsRun.then(result => go.run(result.instance))

self.onmessage = async event => {
	await methodsRun;

	const { value, error } = self[event.data.method](...event.data.args);

	self.postMessage({
		id: event.data.id,
		value,
		error
	});
};
