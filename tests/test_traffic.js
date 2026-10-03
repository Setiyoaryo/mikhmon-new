const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

let requests = [];
const context = { $: { ajax: (options) => requests.push(options) }, isFinite };
vm.createContext(context);
vm.runInContext(fs.readFileSync('js/traffic.js', 'utf8'), context);

let points = [];
let subtitle = '';
const chart = {
  series: [0, 1].map((index) => ({ data: [], addPoint: (point) => points.push([index, ...point]) })),
  setSubtitle: (options) => { subtitle = options.text; },
  redraw: () => {},
};
context.mikhmonRequestTraffic(chart, 'test session', 'WAN & Backup');
assert.equal(requests.length, 1);
assert.equal(requests[0].data.iface, 'WAN & Backup');
assert.equal(requests[0].dataType, 'json');
context.mikhmonRequestTraffic(chart, 'test session', 'WAN & Backup');
assert.equal(requests.length, 1, 'must not overlap pending samples');
requests[0].success([{ name: 'Tx', data: [123456] }, { name: 'Rx', data: [654321] }]);
assert.equal(points[0][2], 123456);
assert.equal(points[1][2], 654321);
requests[0].complete();

points = [];
context.mikhmonRequestTraffic(chart, 'test session', 'WAN & Backup');
requests[1].success([{ data: [null] }, { data: [null] }]);
assert.equal(points.length, 0, 'missing samples must not become zero');
assert.notEqual(subtitle, '');
requests[1].complete();

context.mikhmonRequestTraffic(chart, 'test session', 'WAN & Backup');
requests[2].success([{ data: [0] }, { data: [0] }]);
assert.equal(points[0][2], 0, 'real zero is a valid sample');
requests[2].complete();

requests = [];
context.mikhmonRequestTraffic(chart, 'test session', null);
assert.equal(requests.length, 0, 'no request without an interface');
console.log('traffic regression checks passed');
