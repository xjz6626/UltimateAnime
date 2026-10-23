import { readFileSync } from 'node:fs';
import { createRenderer, nextTick } from 'vue';
import { parse, compileScript } from '@vue/compiler-sfc';

// Compile the actual SFC templates and exercise Vue's event/patch lifecycle.
// The host is in-memory: these tests don't claim to check browser layout.
export async function mountComponent(relativePath, api) {
  const url = new URL(relativePath, import.meta.url);
  const { descriptor } = parse(readFileSync(url, 'utf8'), { filename: url.pathname });
  let { content } = compileScript(descriptor, { id: relativePath, inlineTemplate: true });
  content = content.replace(/from (['"])([^'"]+)\1/g, (_all, _quote, specifier) => {
    const resolved = specifier.startsWith('.') ? new URL(specifier, url).href : import.meta.resolve(specifier);
    return `from ${JSON.stringify(resolved)}`;
  });
  const component = (await import(`data:text/javascript;base64,${Buffer.from(content).toString('base64')}`)).default;
  const events = new Map(), warnings = [], errors = [];
  globalThis.window = { go: { main: { App: api } }, runtime: { EventsOn: (name, callback) => {
    events.set(name, callback); return () => events.delete(name);
  } } };
  const node = (type, text = '') => ({ type, text, props: {}, children: [], parent: null });
  const remove = child => {
    if (child.parent) child.parent.children = child.parent.children.filter(item => item !== child);
    child.parent = null;
  };
  const renderer = createRenderer({
    createElement: type => node(type), createText: text => node('#text', text), createComment: () => node('#comment'),
    setText: (el, text) => { el.text = text; },
    setElementText: (el, text) => { el.text = text; el.children = []; },
    patchProp: (el, key, _old, value) => { el.props[key] = value; },
    insert: (child, parent, anchor) => {
      remove(child); child.parent = parent;
      const index = parent.children.indexOf(anchor);
      if (index < 0) parent.children.push(child); else parent.children.splice(index, 0, child);
    },
    remove, parentNode: el => el.parent,
    nextSibling: el => el.parent?.children[el.parent.children.indexOf(el) + 1],
  });
  const root = node('root'), app = renderer.createApp(component);
  app.config.warnHandler = message => warnings.push(message);
  app.config.errorHandler = error => errors.push(error);
  app.mount(root);
  const flush = async () => { await new Promise(setImmediate); await nextTick(); };
  await flush();
  const text = (el = root) => el.text + el.children.map(text).join('');
  const find = (predicate, el = root) => predicate(el) ? el : el.children.map(child => find(predicate, child)).find(Boolean);
  return { root, events, warnings, errors, flush, text, find,
    button: label => find(el => el.type === 'button' && text(el) === label),
    dispose: () => app.unmount() };
}
