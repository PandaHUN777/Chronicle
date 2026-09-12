// permissions_load_failure.test.mjs — ADR-057 slice 2, the Scribe defect.
//
// static/js/widgets/permissions.js seeds its state with defaults
// (visibility: 'default', isPrivate: false — i.e. "Everyone") and then
// load() fires GET .../permissions, a route that is Owner-only. For a
// Scribe (or anyone else the route 403s) the request fails, the widget
// swallowed the error, and getMode() fell through to the untouched init
// defaults — rendering "Permissions · Everyone" on a page that may in fact
// be DM-only. An actively wrong claim about who can see the page.
//
// The fix: a failed (non-2xx, or network-failed) load must never render a
// mode at all — no mode word in the trigger, no mode badge, no glance dot,
// and no mode-driven body content (picker or read-only badge). Only the
// existing perm-error styling may speak, since it already carries the
// server's real message. Draft mode has no endpoint and is NOT a failed
// load, and must keep showing its (legitimate, local-only) mode.
//
// Harness mirrors test/js/permissions_inline.test.mjs exactly (same
// dependency-free mini-DOM, same vm sandbox) — no new test framework, no
// new dependency, per the slice's constraints.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import vm from 'node:vm';

const here = dirname(fileURLToPath(import.meta.url));
const jsPath = join(here, '..', '..', 'static', 'js', 'widgets', 'permissions.js');

// Minimal DOM: a node whose className and classList share one token set, so
// the widget's mix of `el.className = …` and `el.classList.add(…)` stays
// consistent and queryable. (Copied verbatim from permissions_inline.test.mjs
// — this file does not introduce a shared harness module, matching how that
// file already keeps its own copy rather than factoring one out.)
function makeNode(tag) {
  const classes = new Set();
  const node = {
    tagName: (tag || 'div').toUpperCase(),
    children: [],
    _handlers: {},
    _attrs: {},
    style: {},
    offsetWidth: 0,
    get className() { return Array.from(classes).join(' '); },
    set className(v) { classes.clear(); String(v).split(/\s+/).forEach((c) => c && classes.add(c)); },
    classList: {
      add: (c) => classes.add(c),
      remove: (c) => classes.delete(c),
      contains: (c) => classes.has(c),
      toggle: (c, f) => { const on = f === undefined ? !classes.has(c) : f; if (on) classes.add(c); else classes.delete(c); return on; },
    },
    setAttribute(k, v) { this._attrs[k] = String(v); },
    getAttribute(k) { return k in this._attrs ? this._attrs[k] : null; },
    removeAttribute(k) { delete this._attrs[k]; },
    appendChild(c) { this.children.push(c); c._parent = this; return c; },
    removeChild(c) { const i = this.children.indexOf(c); if (i >= 0) this.children.splice(i, 1); return c; },
    addEventListener(ev, fn) { (this._handlers[ev] = this._handlers[ev] || []).push(fn); },
    removeEventListener() {},
    dispatch(ev) { (this._handlers[ev] || []).forEach((fn) => fn({ key: '', preventDefault() {} })); },
    set innerHTML(_v) { this.children = []; },
    get innerHTML() { return ''; },
    querySelector() { return null; },
    querySelectorAll() { return []; },
    get parentNode() { return this._parent || null; },
  };
  return node;
}

// Recursively find the first descendant carrying a class.
function findByClass(root, cls) {
  for (const c of root.children || []) {
    if (c.classList && c.classList.contains(cls)) return c;
    const deep = findByClass(c, cls);
    if (deep) return deep;
  }
  return null;
}

// Gather all text within a node's subtree (mirrors the "note names the
// subject" helper in permissions_inline.test.mjs).
function gatherText(n) {
  let s = n.textContent || '';
  for (const c of n.children || []) s += ' ' + gatherText(c);
  return s;
}

// boot() wires the widget into the mini-DOM. `fetchResult` lets a test stub
// what Chronicle.apiFetch resolves to — default is a 403, the Owner-only
// route's real response shape for a Scribe.
function boot(fetchResult) {
  const head = makeNode('head');
  const body = makeNode('body');
  const styleRegistry = {};
  const document = {
    getElementById: (id) => styleRegistry[id] || null,
    createElement: (tag) => {
      const n = makeNode(tag);
      Object.defineProperty(n, 'id', { get() { return this._attrs.id; }, set(v) { this._attrs.id = v; styleRegistry[v] = this; } });
      return n;
    },
    head, body,
    addEventListener() {}, removeEventListener() {},
    querySelector() { return null; },
  };
  const apiCalls = [];
  const result = fetchResult || {
    ok: false,
    status: 403,
    json: () => Promise.resolve({ error: 'forbidden', message: 'Only the campaign owner can view permissions.', category: 'auth' }),
  };
  const sandbox = {
    console,
    document,
    AbortController: class { constructor() { this.signal = {}; } abort() {} },
    setTimeout, clearTimeout,
    Chronicle: {
      _impls: {},
      register(name, impl) { this._impls[name] = impl; },
      escapeHtml: (s) => String(s == null ? '' : s),
      apiFetch: (url, opts) => {
        apiCalls.push({ url, opts });
        return Promise.resolve(result);
      },
    },
  };
  sandbox.window = sandbox;
  sandbox.global = sandbox;
  sandbox.__apiCalls = apiCalls;
  vm.createContext(sandbox);
  vm.runInContext(readFileSync(jsPath, 'utf8'), sandbox, { filename: 'permissions.js' });
  return { sandbox, document, body, apiCalls };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

test('a 403 load never shows a mode word in the trigger (the Scribe defect)', async () => {
  const { sandbox, body } = boot();
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { editable: true, endpoint: '/campaigns/c1/entities/e1/permissions' });
  await flush(); // let the 403 rejection resolve and re-render

  const trigger = findByClass(el, 'perm-trigger');
  assert.ok(trigger, 'trigger still renders');
  const text = gatherText(trigger);
  assert.ok(!/Everyone/.test(text), 'trigger must not claim "Everyone" on a failed load: got ' + JSON.stringify(text));
  assert.ok(!/DM Only/.test(text), 'trigger must not claim "DM Only" on a failed load either: got ' + JSON.stringify(text));
  assert.ok(!/Custom/.test(text), 'trigger must not claim "Custom" on a failed load either: got ' + JSON.stringify(text));

  const modeBadge = findByClass(trigger, 'perm-trigger-mode');
  assert.equal(modeBadge, null, 'no perm-trigger-mode badge on a failed load');

  const dot = findByClass(el, 'perm-trigger-widened');
  assert.equal(dot, null, 'no tag-widened glance dot on a failed load — mode is unknown');

  void body;
});

test('a 403 load renders the reused perm-error styling in the body, not the mode picker', async () => {
  // Default (non-inline) layout attaches the card — and its body — to
  // <body>, not to the mount element (see the "default layout ... slide-in
  // card + backdrop on body" regression test in permissions_inline.test.mjs)
  // — so body content is asserted against `body`, mirroring that pattern.
  const { sandbox, body } = boot();
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { editable: true, endpoint: '/campaigns/c1/entities/e1/permissions' });
  await flush();

  const err = findByClass(body, 'perm-error');
  assert.ok(err, 'the existing perm-error region is reused to say the load failed');
  assert.ok(/owner/i.test(gatherText(err)), 'the real server message is shown, not a generic guess: got ' + JSON.stringify(gatherText(err)));

  assert.equal(findByClass(body, 'perm-mode-list'), null, 'no mode picker rendered on a failed load');
});

test('a 403 load renders no mode-driven read-only badge either (non-editable widget)', async () => {
  const { sandbox, body } = boot();
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { editable: false, endpoint: '/campaigns/c1/entities/e1/permissions' });
  await flush();

  const badge = findByClass(body, 'perm-readonly-badge');
  assert.equal(badge, null, 'no read-only "Visible to everyone" badge on a failed load');
  assert.ok(findByClass(body, 'perm-error'), 'the failure is still reported honestly');
});

test('a network-level rejection (not just a 403 body) is treated the same as a failed load', async () => {
  const { sandbox, body } = boot({ ok: false, status: 500, json: () => Promise.reject(new Error('bad json')) });
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { editable: true, endpoint: '/campaigns/c1/entities/e1/permissions' });
  await flush();

  const trigger = findByClass(el, 'perm-trigger');
  assert.ok(!/Everyone/.test(gatherText(trigger)), 'still no guessed mode on a non-JSON failure');
  assert.equal(findByClass(body, 'perm-mode-list'), null, 'still no mode picker');
});

test('draft mode is NOT treated as a failed load and keeps showing its mode', async () => {
  const { sandbox, body, apiCalls } = boot();
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { mode: 'draft', draftTarget: '#is_private', editable: true });
  await flush();

  assert.equal(apiCalls.length, 0, 'draft mode never calls the network');
  const trigger = findByClass(el, 'perm-trigger');
  assert.ok(/Everyone/.test(gatherText(trigger)), 'draft mode legitimately starts as Everyone, and must still say so');
  assert.equal(findByClass(body, 'perm-error'), null, 'draft mode has no endpoint — that is not a failure');
});

test('a successful load still shows the real mode (regression guard)', async () => {
  const { sandbox, body } = boot({
    ok: true,
    json: () => Promise.resolve({ visibility: 'default', is_private: true, members: [], groups: [], permissions: [] }),
  });
  const impl = sandbox.Chronicle._impls.permissions;
  const el = makeNode('div');
  impl.init(el, { editable: true, endpoint: '/campaigns/c1/entities/e1/permissions' });
  await flush();

  const trigger = findByClass(el, 'perm-trigger');
  assert.ok(/DM Only/.test(gatherText(trigger)), 'a successful load renders the real mode: got ' + JSON.stringify(gatherText(trigger)));
  assert.equal(findByClass(body, 'perm-error'), null, 'no error banner on success');
});
