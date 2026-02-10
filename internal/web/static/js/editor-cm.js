/**
 * Docstor Editor - CodeMirror 6 with vim mode and draft saving
 * 
 * Uses a bundled CodeMirror 6 (codemirror-bundle.js) for the editor,
 * with localStorage draft saving for crash recovery.
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

// Vim mode state
let vimEnabled = localStorage.getItem('docstor_vim_mode') === 'true';

function createVimToggle(editor) {
    const container = document.createElement('div');
    container.className = 'editor-toolbar';
    container.innerHTML = `
        <label class="vim-toggle">
            <input type="checkbox" id="vim-mode-toggle" ${vimEnabled ? 'checked' : ''}>
            <span>Vim mode</span>
        </label>
        <span class="vim-indicator" id="vim-indicator" style="display: ${vimEnabled ? 'inline' : 'none'}">
            Press <kbd>Esc</kbd> then <kbd>:</kbd> for commands, <kbd>i</kbd> to insert
        </span>
    `;
    return container;
}

function initCodeMirrorEditor() {
    const textarea = document.getElementById('body');
    if (!textarea || typeof DocstorEditor === 'undefined') {
        // Fallback to enhanced textarea
        initEnhancedTextarea();
        return;
    }
    
    const currentContent = textarea.value;
    const draft = loadDraft();
    
    // Hide original textarea
    textarea.style.display = 'none';
    
    // Create editor container
    const editorContainer = document.createElement('div');
    editorContainer.id = 'cm-editor-container';
    editorContainer.style.border = '1px solid #d0d7de';
    editorContainer.style.borderRadius = '6px';
    editorContainer.style.overflow = 'hidden';
    editorContainer.style.minHeight = '400px';
    textarea.parentNode.insertBefore(editorContainer, textarea);
    
    // Create CodeMirror editor
    const editor = DocstorEditor.createEditor(editorContainer, {
        content: currentContent,
        vim: vimEnabled,
        dark: false,  // Docstor is light-mode; user can toggle via future dark mode preference
        onUpdate: (content) => {
            textarea.value = content;
            saveDraft(content);
        }
    });
    
    // Add toolbar above editor
    const toolbar = createVimToggle(editor);
    editorContainer.parentNode.insertBefore(toolbar, editorContainer);
    
    // Vim toggle handler
    document.getElementById('vim-mode-toggle').addEventListener('change', (e) => {
        vimEnabled = e.target.checked;
        localStorage.setItem('docstor_vim_mode', vimEnabled);
        editor.toggleVim(vimEnabled);
        document.getElementById('vim-indicator').style.display = vimEnabled ? 'inline' : 'none';
    });
    
    // Draft recovery
    if (draft) {
        showDraftPrompt(draft, currentContent,
            content => {
                editor.setContent(content);
                textarea.value = content;
            },
            clearDraft
        );
    }
    
    // Clear draft and sync content on form submit
    textarea.closest('form')?.addEventListener('submit', () => {
        textarea.value = editor.getContent();
        clearDraft();
    });
    
    // Focus editor
    editor.focus();
    
    console.log('CodeMirror 6 editor initialized' + (vimEnabled ? ' (vim mode)' : ''));
}

function initEnhancedTextarea() {
    const textarea = document.getElementById('body');
    if (!textarea) return;
    
    const currentContent = textarea.value;
    const draft = loadDraft();
    
    // Enhanced textarea styling
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

function initEditor() {
    // Try CodeMirror first, fall back to enhanced textarea
    if (typeof DocstorEditor !== 'undefined') {
        initCodeMirrorEditor();
    } else {
        initEnhancedTextarea();
    }
}

// Init when ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initEditor);
} else {
    initEditor();
}
