const worker = new Worker('worker.js');

const callbacks = {};
let id = 0;

async function runMethod(method, args) {
	return new Promise((resolve, reject) => {
		callbacks[id] = ({ value, error }) => {
			if(error) {
				reject(new Error(error));
			} else {
				resolve(value);
			}
		};

		worker.postMessage({
			method,
			args,
			id: id++
		});
	});
}

worker.onmessage = async function handleCallback(event) {
	callbacks[event.data.id](event.data);
};

module.exports = new Proxy({}, {
	get: (target, prop, receiver) =>
		async (...args) =>
			runMethod(prop, args)
});
