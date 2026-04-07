// === Copy post markdown to clipboard ===
function getCookie(name) {
    var prefix = name + '=';
    var cookies = document.cookie ? document.cookie.split('; ') : [];
    for (var i = 0; i < cookies.length; i++) {
        if (cookies[i].indexOf(prefix) === 0) {
            return decodeURIComponent(cookies[i].slice(prefix.length));
        }
    }
    return '';
}

function getCSRFToken() {
    return getCookie('csrf_token');
}

function absolutizeCopiedMarkdown(text) {
    if (!text) return text;
    var origin = window.location.origin || '';
    if (!origin) return text;
    return text
        .replace(/\]\(\/(?!\/)/g, '](' + origin + '/')
        .replace(/^(\[[^\]]+\]:)\s+\/(?!\/)/gm, '$1 ' + origin + '/');
}

function copyMarkdown(btn, postId, revisionNumber) {
    if (!postId && btn && btn.dataset) {
        postId = btn.dataset.copyPostId;
    }
    if (revisionNumber == null && btn && btn.dataset && btn.dataset.copyRevision) {
        revisionNumber = btn.dataset.copyRevision;
    }

    var sourceId = btn && btn.dataset ? btn.dataset.copySource : '';
    if (sourceId) {
        var source = document.getElementById(sourceId);
        if (source) {
            var text = source.content ? source.content.textContent : source.textContent;
            writeTextToClipboard(absolutizeCopiedMarkdown(text || ''))
                .then(function() { showCopyFeedback(btn); })
                .catch(function() { alert('Failed to copy'); });
            return;
        }
    }

    let url = '/posts/' + postId + '/raw';
    if (revisionNumber) {
        url += '?revision=' + encodeURIComponent(revisionNumber);
    }
    copyTextFromURL(btn, url);
}

function writeTextFallback(text) {
    var textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.top = '-9999px';
    textarea.style.left = '-9999px';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    try {
        if (document.execCommand('copy')) {
            document.body.removeChild(textarea);
            return true;
        }
    } catch (e) {
    }
    document.body.removeChild(textarea);
    return false;
}

function writeTextToClipboard(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
        return navigator.clipboard.writeText(text);
    }
    return new Promise(function(resolve, reject) {
        if (writeTextFallback(text)) {
            resolve();
            return;
        }
        reject(new Error('copy-failed'));
    });
}

function copyTextFromURL(btn, url) {
    var request = fetch(url, { credentials: 'same-origin' }).then(function(r) {
        if (!r.ok) throw new Error('fetch-failed');
        return r.text().then(absolutizeCopiedMarkdown);
    });

    if (window.ClipboardItem && navigator.clipboard && navigator.clipboard.write) {
        navigator.clipboard.write([
            new ClipboardItem({
                'text/plain': request.then(function(text) {
                    return new Blob([text], { type: 'text/plain' });
                }),
            }),
        ])
            .then(function() {
                showCopyFeedback(btn);
            })
            .catch(function() {
                request
                    .then(function(text) { return writeTextToClipboard(text); })
                    .then(function() { showCopyFeedback(btn); })
                    .catch(function() { alert('Failed to copy'); });
            });
        return;
    }

    request
        .then(function(text) { return writeTextToClipboard(text); })
        .then(function() { showCopyFeedback(btn); })
        .catch(function() { alert('Failed to copy'); });
}

function showCopyFeedback(btn) {
    var icon = btn.querySelector('.icon');
    if (icon) {
        var orig = icon.innerHTML;
        icon.innerHTML = '<polyline points="20 6 9 17 4 12"/>';
        setTimeout(function() { icon.innerHTML = orig; }, 1500);
        return;
    }
    var origText = btn.dataset.originalText || btn.textContent;
    if (!btn.dataset.originalText) {
        btn.dataset.originalText = origText;
    }
    btn.textContent = 'Copied';
    setTimeout(function() { btn.textContent = btn.dataset.originalText; }, 1500);
}

function withCSRFHeaders(headers) {
    var token = getCSRFToken();
    if (!token) return headers;
    headers = headers || {};
    headers['X-CSRF-Token'] = token;
    return headers;
}

function ensureCSRFInputs(root) {
    (root || document).querySelectorAll('form[method="POST"], form[method="post"]').forEach(function(form) {
        var input = form.querySelector('input[name="csrf_token"]');
        if (!input) {
            input = document.createElement('input');
            input.type = 'hidden';
            input.name = 'csrf_token';
            form.appendChild(input);
        }
        input.value = getCSRFToken();
    });
}

function getExpandableBody(button) {
    var el = button ? button.previousElementSibling : null;
    while (el) {
        if (el.classList && el.classList.contains('post-body-preview')) {
            return el;
        }
        el = el.previousElementSibling;
    }
    return null;
}

function collapseExpandableBody(body) {
    body.style.maxHeight = body.scrollHeight + 'px';
    body.classList.remove('expanded');
    body.offsetHeight;
    body.style.setProperty('--expanded-max-height', body.scrollHeight + 'px');
    body.style.maxHeight = '';
}

function expandExpandableBody(body) {
    var collapsedHeight = body.offsetHeight;
    body.style.maxHeight = collapsedHeight + 'px';
    body.classList.add('expanded');
    body.style.setProperty('--expanded-max-height', body.scrollHeight + 'px');
    body.offsetHeight;
    body.style.maxHeight = body.scrollHeight + 'px';
}

function syncExpandedHeight(body) {
    if (!body.classList.contains('expanded')) {
        body.style.removeProperty('--expanded-max-height');
        body.style.removeProperty('max-height');
        return;
    }
    var expandedHeight = body.scrollHeight;
    body.style.setProperty('--expanded-max-height', expandedHeight + 'px');
    body.style.maxHeight = expandedHeight + 'px';
}

function syncExpandablePosts(root) {
    (root || document).querySelectorAll('.continue-link').forEach(function(button) {
        var body = getExpandableBody(button);
        if (!body) {
            button.hidden = true;
            return;
        }

        if (body.classList.contains('expanded')) {
            button.hidden = false;
            syncExpandedHeight(body);
            return;
        }

        var collapsedHeight = body.clientHeight;
        body.classList.add('expanded');
        var expandedHeight = body.scrollHeight;
        body.classList.remove('expanded');

        if (expandedHeight <= collapsedHeight + 24) {
            body.classList.remove('post-body-preview');
            button.hidden = true;
            body.style.removeProperty('--expanded-max-height');
            body.style.removeProperty('max-height');
        } else {
            button.hidden = false;
        }
    });
}

function syncWelcomeBanner() {
    var banner = document.querySelector('[data-welcome-banner]');
    if (!banner) return;
    var dismissed = false;
    try {
        dismissed = window.localStorage.getItem('kt-welcome-banner-dismissed') === '1';
    } catch (e) {
    }
    banner.hidden = dismissed;
}

document.addEventListener('DOMContentLoaded', function() {
    ensureCSRFInputs(document);
    syncExpandablePosts(document);
    syncWelcomeBanner();
});

document.body.addEventListener('htmx:afterSwap', function(e) {
    ensureCSRFInputs(e.target);
    syncExpandablePosts(e.target);
    syncWelcomeBanner();
});

document.body.addEventListener('htmx:configRequest', function(e) {
    var token = getCSRFToken();
    if (!token) return;
    e.detail.headers['X-CSRF-Token'] = token;
});

// === Tab switching (Write / Preview) ===
document.addEventListener('click', function(e) {
    const copyButton = e.target.closest('.copy-btn, .docs-icon-btn[data-copy-url]');
    if (copyButton) {
        if (copyButton.dataset.copyUrl) {
            copyTextFromURL(copyButton, copyButton.dataset.copyUrl);
        } else {
            copyMarkdown(copyButton);
        }
        return;
    }

    const continueLink = e.target.closest('.continue-link');
    if (continueLink) {
        const body = getExpandableBody(continueLink);
        if (body) {
            const expanded = !body.classList.contains('expanded');
            if (expanded) {
                expandExpandableBody(body);
            } else {
                collapseExpandableBody(body);
            }
            continueLink.textContent = expanded
                ? (continueLink.dataset.collapseLabel || 'Show less')
                : (continueLink.dataset.expandLabel || 'Continue reading');
        }
        return;
    }

    const dismissWelcome = e.target.closest('[data-dismiss-welcome-banner]');
    if (dismissWelcome) {
        var banner = dismissWelcome.closest('[data-welcome-banner]');
        if (banner) {
            banner.hidden = true;
        }
        try {
            window.localStorage.setItem('kt-welcome-banner-dismissed', '1');
        } catch (err) {
        }
        return;
    }

    const tab = e.target.closest('.editor-tabs .tab');
    if (!tab) return;

    const editor = tab.closest('.editor');
    const tabs = editor.querySelectorAll('.editor-tabs .tab');
    const isPreview = tab.dataset.tab === 'preview';

    // Update tab styles
    tabs.forEach(t => t.classList.remove('active'));
    tab.classList.add('active');

    // Toggle content
    const writeTab = editor.querySelector('.write-tab');
    const previewTab = editor.querySelector('.preview-tab');

    if (isPreview) {
        writeTab.classList.remove('active');
        previewTab.classList.add('active');

        // Fetch rendered preview
        const textarea = writeTab.querySelector('textarea');
        const previewArea = previewTab.querySelector('.preview-area');
        if (textarea && textarea.value.trim()) {
            previewArea.innerHTML = '<p style="color:#656d76">Loading preview...</p>';
            fetch('/preview', {
                method: 'POST',
                headers: withCSRFHeaders({'Content-Type': 'application/x-www-form-urlencoded'}),
                body: 'content=' + encodeURIComponent(textarea.value)
            })
            .then(r => r.text())
            .then(html => { previewArea.innerHTML = html; })
            .catch(() => { previewArea.innerHTML = '<p style="color:#d1242f">Preview failed</p>'; });
        } else {
            previewArea.innerHTML = '<p style="color:#656d76">Nothing to preview</p>';
        }
    } else {
        previewTab.classList.remove('active');
        writeTab.classList.add('active');
    }
});

document.addEventListener('submit', function(e) {
    const form = e.target.closest('form[data-confirm]');
    if (!form) return;
    if (!window.confirm(form.dataset.confirm)) {
        e.preventDefault();
    }
});

window.addEventListener('resize', function() {
    document.querySelectorAll('.post-body-preview.expanded').forEach(function(body) {
        syncExpandedHeight(body);
    });
});

document.addEventListener('transitionend', function(e) {
    if (!e.target.classList || !e.target.classList.contains('post-body-preview')) {
        return;
    }
    if (e.propertyName !== 'max-height') {
        return;
    }
    if (e.target.classList.contains('expanded')) {
        e.target.style.maxHeight = 'none';
    }
});

function updateTextareaCount(textarea) {
    const editor = textarea.closest('.editor');
    if (!editor) return;

    const charCount = editor.querySelector('.char-count');
    if (!charCount) return;

    const limit = textarea.maxLength > 0 ? textarea.maxLength : 0;
    const len = textarea.value.length;
    charCount.textContent = limit > 0
        ? len.toLocaleString() + ' / ' + limit.toLocaleString()
        : len.toLocaleString();
    charCount.style.color = limit > 0 && len > Math.floor(limit * 0.95) ? '#d1242f' : '';
}

// === Image paste & drag-drop ===
document.querySelectorAll('.editor textarea').forEach(function(textarea) {
    textarea.addEventListener('input', function() {
        updateTextareaCount(textarea);
    });
    updateTextareaCount(textarea);

    // Paste
    textarea.addEventListener('paste', function(e) {
        const items = e.clipboardData && e.clipboardData.items;
        if (!items) return;
        for (let i = 0; i < items.length; i++) {
            if (items[i].type.indexOf('image') !== -1) {
                e.preventDefault();
                uploadImage(items[i].getAsFile(), textarea);
                return;
            }
        }
    });

    // Drag & drop
    textarea.addEventListener('dragover', function(e) {
        e.preventDefault();
        textarea.classList.add('drag-over');
    });
    textarea.addEventListener('dragleave', function() {
        textarea.classList.remove('drag-over');
    });
    textarea.addEventListener('drop', function(e) {
        e.preventDefault();
        textarea.classList.remove('drag-over');
        const files = e.dataTransfer.files;
        for (let i = 0; i < files.length; i++) {
            if (files[i].type.startsWith('image/')) {
                uploadImage(files[i], textarea);
            }
        }
    });
});

function uploadImage(file, textarea) {
    if (file.size > 5 * 1024 * 1024) {
        alert('Image too large (max 5MB)');
        return;
    }

    // Insert placeholder
    const placeholder = '![Uploading...]()';
    const start = textarea.selectionStart;
    textarea.value = textarea.value.slice(0, start) + placeholder + textarea.value.slice(start);

    const formData = new FormData();
    formData.append('image', file);

    fetch('/upload', { method: 'POST', body: formData, headers: withCSRFHeaders() })
        .then(r => {
            if (!r.ok) throw new Error('Upload failed');
            return r.json();
        })
        .then(data => {
            textarea.value = textarea.value.replace(placeholder, data.markdown);
            updateTextareaCount(textarea);
        })
        .catch(() => {
            textarea.value = textarea.value.replace(placeholder, '');
            alert('Image upload failed');
        });
}
