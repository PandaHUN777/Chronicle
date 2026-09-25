/**
 * markdown_importer.js — drag/drop + multi-file upload widget for
 * AI Workspace > Import.
 *
 * Mount: data-widget="markdown-importer", using child elements
 * data-importer-dropzone, data-importer-file-input (keyboard fallback),
 * data-importer-file-list, data-importer-file-announce (sr-only aria-live).
 *
 * Owns only the drag/preview UI (drag-active state, file list, a11y
 * announcements). Submission is the surrounding HTMX <form>'s job — the
 * widget never POSTs.
 */
(function () {
  'use strict';

  Chronicle.register('markdown-importer', {
    init: function (el) {
      var zone = el.querySelector('[data-importer-dropzone]');
      var input = el.querySelector('[data-importer-file-input]');
      var list = el.querySelector('[data-importer-file-list]');
      var announce = el.querySelector('[data-importer-file-announce]');
      if (!zone || !input || !list) {
        return;
      }

      ['dragenter', 'dragover'].forEach(function (evt) {
        zone.addEventListener(evt, function (e) {
          e.preventDefault();
          e.stopPropagation();
          zone.classList.add('ring-2', 'ring-accent', 'bg-accent/5');
          announceText(announce, 'Drop to attach markdown files.');
        });
      });
      ['dragleave', 'drop'].forEach(function (evt) {
        zone.addEventListener(evt, function (e) {
          e.preventDefault();
          e.stopPropagation();
          zone.classList.remove('ring-2', 'ring-accent', 'bg-accent/5');
        });
      });

      zone.addEventListener('drop', function (e) {
        var dropped = Array.from(e.dataTransfer.files || []).filter(function (f) {
          var n = f.name.toLowerCase();
          return n.endsWith('.md') || n.endsWith('.markdown');
        });
        if (dropped.length === 0) {
          announceText(announce, 'No markdown files in that drop.');
          return;
        }
        // Replace the input's FileList with the dropped files.
        // FileList is read-only; build a DataTransfer to assign.
        var dt = new DataTransfer();
        dropped.forEach(function (f) { dt.items.add(f); });
        input.files = dt.files;
        renderList(dropped, list);
        announceText(announce, dropped.length + ' file(s) attached.');
      });

      input.addEventListener('change', function () {
        var files = Array.from(input.files || []);
        renderList(files, list);
        if (files.length > 0) {
          announceText(announce, files.length + ' file(s) selected.');
        }
      });
    },
  });

  function announceText(target, msg) {
    if (!target) { return; }
    target.textContent = msg;
  }

  function renderList(files, list) {
    list.innerHTML = '';
    if (files.length === 0) {
      return;
    }
    files.forEach(function (f) {
      var li = document.createElement('div');
      li.innerHTML = '<i class="fa-solid fa-file-code mr-1" aria-hidden="true"></i> ' +
        escapeHtml(f.name) +
        ' <span class="text-fg-muted">(' + (f.size / 1024).toFixed(1) + ' KB)</span>';
      list.appendChild(li);
    });
  }

  function escapeHtml(s) {
    return s.replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }
})();
