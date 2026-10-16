import test from 'node:test';
import assert from 'node:assert/strict';
import { StreamEpisodeMagnets } from './api.js';

class FakeEventSource extends EventTarget {
  static OPEN = 1;
  static latest;
  readyState = 0;
  constructor() { super(); FakeEventSource.latest = this; }
  open() { this.readyState = 1; this.dispatchEvent(new Event('open')); }
  receive(data) { this.onmessage({ data: JSON.stringify({ name: 'magnet-search-results', data }) }); }
  close() { this.readyState = 2; }
}

test('web search displays partial results before RPC completes and ignores other requests', async () => {
  globalThis.window = { __ULTIMATEANIME_WEB__: true };
  globalThis.EventSource = FakeEventSource;
  let finish;
  let requestID;
  globalThis.fetch = async (_url, options) => {
    const request = JSON.parse(options.body);
    requestID = request.args[3];
    return new Promise((resolve) => {
      finish = (items) => resolve({ ok: true, json: async () => ({ result: items }) });
    });
  };
  const updates = [];
  const pending = StreamEpisodeMagnets(42, 1, '', (items) => updates.push(items));
  const events = FakeEventSource.latest;
  assert.equal(requestID, undefined, 'RPC must wait until SSE subscribes');
  events.open();
  await new Promise(setImmediate);
  const first = [{ magnet: 'magnet:first' }];
  events.receive({ request_id: 'another-browser', items: [{ magnet: 'wrong' }] });
  events.receive({ request_id: requestID, items: first });
  assert.deepEqual(updates, [first]);
  const final = [...first, { magnet: 'magnet:second' }];
  finish(final);
  assert.deepEqual(await pending, final);
  assert.deepEqual(updates, [first, final]);
  assert.equal(events.readyState, 2, 'search listener was not released');
});

test('canceling desktop search ignores late events and results', async () => {
  let handler;
  let finish;
  let requestID;
  let cancellations = 0;
  let unsubscribed = false;
  globalThis.window = {
    __ULTIMATEANIME_WEB__: false,
    runtime: { EventsOn: (_name, callback) => { handler = callback; return () => { unsubscribed = true; }; } },
    go: { main: { App: {
      SearchEpisodeMagnetListStream: (_id, _episode, _keywords, id) => {
        requestID = id;
        return new Promise((resolve) => { finish = resolve; });
      },
      CancelEpisodeMagnetSearch: (id) => { assert.equal(id, requestID); cancellations++; },
    } } },
  };
  const updates = [];
  const controller = new AbortController();
  const pending = StreamEpisodeMagnets(42, 1, '', (items) => updates.push(items), controller.signal);
  await new Promise(setImmediate);
  controller.abort();
  handler({ request_id: requestID, items: [{ magnet: 'late' }] });
  finish([{ magnet: 'late' }]);
  await pending;
  assert.deepEqual(updates, []);
  assert.ok(cancellations > 0);
  assert.ok(unsubscribed);
});

test('canceling web search aborts HTTP and releases its event subscription', async () => {
  globalThis.window = { __ULTIMATEANIME_WEB__: true };
  globalThis.EventSource = FakeEventSource;
  let requestSignal;
  globalThis.fetch = async (_url, options) => {
    if (JSON.parse(options.body).method === 'CancelEpisodeMagnetSearch') {
      return { ok: true, json: async () => ({ result: null }) };
    }
    requestSignal = options.signal;
    return new Promise((_resolve, reject) => {
      requestSignal.addEventListener('abort', () => reject(new DOMException('Canceled', 'AbortError')), { once: true });
    });
  };
  const controller = new AbortController();
  const pending = StreamEpisodeMagnets(42, 1, '', () => assert.fail('canceled results were displayed'), controller.signal);
  const events = FakeEventSource.latest;
  events.open();
  await new Promise(setImmediate);
  controller.abort();
  await assert.rejects(pending, { name: 'AbortError' });
  assert.equal(requestSignal.aborted, true);
  assert.equal(events.readyState, 2);
});
