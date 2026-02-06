/**
 * Docstor Editor - Enhanced textarea with draft saving
 * CodeMirror 6 is complex to load from CDN due to module dependencies.
 * This provides a good editing experience with textarea + draft saving.
 * 
 * TODO: For CodeMirror 6, build a proper bundle with esbuild/rollup
 */

// Draft storage
const DRAFT_PREFIX = 'docstor_draft_';
const DRAFT_TS_SUFFIX = '_ts';

function getDraftKey() {
    const form = document.querySelector('form[action*="/docs/id/"]');
    if (form) {
        const match = form.action.match(/\/docs\/id\/([^/]+)/);
        if (match) return DRAFT_PREFIX + match[1];
    }
    return DRAFT_PREFIX + window.location.pathname;
}

function saveDraft(content) {
    try {
        const key = getDraftKey();
        localStorage.setItem(key, content);
        localStorage.setItem(key + DRAFT_TS_SUFFIX, Date.now().toString());
        updateDraftIndicator(true);
    } catch (e) { /* ignore */ }
}

function loadDraft() {
    try {
        const key = getDraftKey();
        const content = localStorage.getItem(key);
        const ts = localStorage.getItem(key + DRAFT_TS_SUFFIX);
        if (content && ts) return { content, timestamp: parseInt(ts, 10) };
    } catch (e) { /* ignore */ }
    return null;
}

function clearDraft() {
    try {
        const key = getDraftKey();
        localStorage.removeItem(key);
        localStorage.removeItem(key + DRAFT_TS_SUFFIX);
        updateDraftIndicator(false);
    } catch (e) { /* ignore */ }
}

function updateDraftIndicator(hasDraft) {
    let el = document.getElementById('draft-indicator');
    if (!el) {
        el = document.createElement('span');
        el.id = 'draft-indicator';
        el.className = 'draft-indicator';
        const actions = document.querySelector('.form-actions');
        if (actions) actions.insertBefore(el, actions.firstChild);
    }
    el.textContent = hasDraft ? 'Draft saved' : '';
    el.style.display = hasDraft ? 'inline' : 'none';
}

function showDraftPrompt(draft, currentContent, onRecover, onDiscard) {
    if (draft.content === currentContent) { clearDraft(); return; }
    
    const banner = document.createElement('div');
    banner.className = 'alert alert-warning draft-recovery';
    const date = new Date(draft.timestamp).toLocaleString();
    banner.innerHTML = `
        <strong>Draft found</strong> from ${date}.
        <button type="button" class="btn btn-sm btn-primary" id="recover-draft">Restore</button>
        <button type="button" class="btn btn-sm btn-secondary" id="discard-draft">Discard</button>
    `;
    
    const header = document.querySelector('.page-header');
    if (header) header.after(banner);
    
    document.getElementById('recover-draft').onclick = () => { onRecover(draft.content); banner.remove(); };
    document.getElementById('discard-draft').onclick = () => { onDiscard(); banner.remove(); };
}

function initEditor() {
    const textarea = document.getElementById('body');
    if (!textarea) return;
    
    const currentContent = textarea.value;
    const draft = loadDraft();
    
    // Enhanced textarea
    textarea.style.fontFamily = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';
    textarea.style.fontSize = '14px';
    textarea.style.lineHeight = '1.5';
    textarea.style.tabSize = '4';
    
    // Tab key support
    textarea.addEventListener('keydown', e => {
        if (e.key === 'Tab') {
            e.preventDefault();
            const start = textarea.selectionStart;
            const end = textarea.selectionEnd;
            textarea.value = textarea.value.substring(0, start) + '    ' + textarea.value.substring(end);
            textarea.selectionStart = textarea.selectionEnd = start + 4;
            saveDraft(textarea.value);
        }
    });
    
    // Save draft on input
    textarea.addEventListener('input', () => saveDraft(textarea.value));
    
    // Draft recovery
    if (draft) {
        showDraftPrompt(draft, currentContent,
            content => { textarea.value = content; },
            clearDraft
        );
    }
    
    // Clear draft on submit
    textarea.closest('form')?.addEventListener('submit', clearDraft);
    
    console.log('Editor initialized (enhanced textarea with draft saving)');
}

// Init when ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initEditor);
} else {
    initEditor();
}
