// availability_responsive.test.mjs — pins that the month/week grids stay usable
// at phone widths: a `@media (max-width:640px)` block gives the grids a legible
// min-width, and each grid renders inside an `.avail-scroll` (overflow-x:auto)
// container so it scrolls horizontally inside its own box instead of crushing
// the page sideways. Pins the containment/CSS contract only; the label
// encoding is covered by availability_labels.test.mjs.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { boot, wait } from './availability_harness.mjs';

function css(doc) { return doc.querySelector('#avail-styles').textContent; }

test('injects a phone-width media query (not just reduced-motion)', () => {
  const { doc } = boot();
  const c = css(doc);
  assert.match(c, /@media \(max-width:640px\)/, 'a max-width phone breakpoint is present');
  // Sanity: the pre-fix file had exactly one @media (reduced-motion). Require >= 2.
  const count = (c.match(/@media/g) || []).length;
  assert.ok(count >= 2, `expected >=2 @media blocks, got ${count}`);
});

test('the phone breakpoint sets a legible min-width on the week + month grids', () => {
  const { doc } = boot();
  const c = css(doc);
  assert.match(c, /\.avail-grid,\.ov-weekgrid\{min-width:\d+px\}/, 'week/my grids get a min-width');
  assert.match(c, /\.ov-month\{min-width:\d+px\}/, 'month grid gets a min-width');
  assert.match(c, /\.avail-scroll\{overflow-x:auto/, 'a horizontal scroll container class exists');
});

test('the my-availability grid renders inside an .avail-scroll container', async () => {
  const { doc } = boot();
  await wait(30);
  assert.ok(
    doc.querySelector('[data-avail-panel="mine"] .avail-scroll .avail-grid'),
    'the paint grid is wrapped in a horizontal-scroll container',
  );
});

test('the week overlay grid renders inside .avail-scroll and carries the ov-weekgrid class', async () => {
  const { doc } = boot();
  await wait(20);
  doc.querySelector('[data-avail-tab="overlay"]').click();
  await wait(40);
  assert.ok(
    doc.querySelector('[data-ov-grid] .avail-scroll .ov-weekgrid'),
    'the week heatmap grid is wrapped in a scroll container and tagged for the min-width rule',
  );
});

test('the month grid renders inside an .avail-scroll container', async () => {
  const { doc } = boot();
  await wait(20);
  doc.querySelector('[data-avail-tab="overlay"]').click();
  await wait(40);
  doc.querySelector('[data-ov-scale="month"]').click();
  await wait(60);
  assert.ok(
    doc.querySelector('[data-ov-month] .avail-scroll .ov-month'),
    'the month grid is wrapped in a horizontal-scroll container',
  );
});
