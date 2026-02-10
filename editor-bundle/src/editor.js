/**
 * Docstor CodeMirror 6 Editor Bundle
 * 
 * Bundles CM6 with markdown-aware syntax highlighting,
 * vim mode, and light/dark themes.
 */

import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection, rectangularSelection, crosshairCursor, dropCursor } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { markdown, markdownLanguage } from '@codemirror/lang-markdown';
import { syntaxHighlighting, HighlightStyle, defaultHighlightStyle, bracketMatching, foldGutter, indentOnInput } from '@codemirror/language';
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
import { vim, Vim } from '@replit/codemirror-vim';
import { tags } from '@lezer/highlight';

// ── Markdown-aware highlight style (light) ──────────────────────
const markdownHighlightLight = HighlightStyle.define([
    // Headings — distinct blue, bold, sized
    { tag: tags.heading1, color: '#1a56db', fontWeight: '700', fontSize: '1.4em' },
    { tag: tags.heading2, color: '#1e40af', fontWeight: '700', fontSize: '1.25em' },
    { tag: tags.heading3, color: '#1e3a8a', fontWeight: '600', fontSize: '1.15em' },
    { tag: tags.heading4, color: '#1e3a8a', fontWeight: '600' },
    { tag: tags.heading5, color: '#1e3a8a', fontWeight: '600' },
    { tag: tags.heading6, color: '#1e3a8a', fontWeight: '600' },
    // Emphasis
    { tag: tags.emphasis, fontStyle: 'italic', color: '#6b21a8' },
    { tag: tags.strong, fontWeight: 'bold', color: '#9a3412' },
    // Links
    { tag: tags.link, color: '#2563eb', textDecoration: 'underline' },
    { tag: tags.url, color: '#0369a1' },
    // Code / monospace
    { tag: tags.monospace, color: '#be123c', backgroundColor: 'rgba(220, 38, 38, 0.06)', borderRadius: '3px', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
    // Quotes
    { tag: tags.quote, color: '#4b5563', fontStyle: 'italic' },
    // Lists
    { tag: tags.list, color: '#059669' },
    // Meta / processing (the #, ##, -, *, ``` markers)
    { tag: tags.processingInstruction, color: '#9ca3af' },
    { tag: tags.meta, color: '#9ca3af' },
    // Strikethrough
    { tag: tags.strikethrough, textDecoration: 'line-through', color: '#6b7280' },
    // Generic content
    { tag: tags.content, color: '#1f2937' },
    // Punctuation / separator (horizontal rules, etc.)
    { tag: tags.separator, color: '#d1d5db' },
    // HTML in markdown
    { tag: tags.angleBracket, color: '#9ca3af' },
    { tag: tags.tagName, color: '#0891b2' },
    { tag: tags.attributeName, color: '#059669' },
    { tag: tags.attributeValue, color: '#be123c' },
    // Comments (HTML comments)
    { tag: tags.comment, color: '#9ca3af', fontStyle: 'italic' },
]);

// ── Markdown-aware highlight style (dark) ───────────────────────
const markdownHighlightDark = HighlightStyle.define([
    { tag: tags.heading1, color: '#60a5fa', fontWeight: '700', fontSize: '1.4em' },
    { tag: tags.heading2, color: '#93bbfd', fontWeight: '700', fontSize: '1.25em' },
    { tag: tags.heading3, color: '#a5b4fc', fontWeight: '600', fontSize: '1.15em' },
    { tag: tags.heading4, color: '#a5b4fc', fontWeight: '600' },
    { tag: tags.heading5, color: '#a5b4fc', fontWeight: '600' },
    { tag: tags.heading6, color: '#a5b4fc', fontWeight: '600' },
    { tag: tags.emphasis, fontStyle: 'italic', color: '#c084fc' },
    { tag: tags.strong, fontWeight: 'bold', color: '#fb923c' },
    { tag: tags.link, color: '#60a5fa', textDecoration: 'underline' },
    { tag: tags.url, color: '#38bdf8' },
    { tag: tags.monospace, color: '#f472b6', backgroundColor: 'rgba(244, 114, 182, 0.1)', borderRadius: '3px', fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
    { tag: tags.quote, color: '#9ca3af', fontStyle: 'italic' },
    { tag: tags.list, color: '#34d399' },
    { tag: tags.processingInstruction, color: '#6b7280' },
    { tag: tags.meta, color: '#6b7280' },
    { tag: tags.strikethrough, textDecoration: 'line-through', color: '#9ca3af' },
    { tag: tags.content, color: '#d1d5db' },
    { tag: tags.separator, color: '#4b5563' },
    { tag: tags.angleBracket, color: '#6b7280' },
    { tag: tags.tagName, color: '#22d3ee' },
    { tag: tags.attributeName, color: '#34d399' },
    { tag: tags.attributeValue, color: '#f472b6' },
    { tag: tags.comment, color: '#6b7280', fontStyle: 'italic' },
]);

// ── Light theme ─────────────────────────────────────────────────
const lightTheme = EditorView.theme({
    '&': {
        backgroundColor: '#ffffff',
        color: '#1f2937',
        fontSize: '14px',
    },
    '.cm-content': {
        caretColor: '#1f2937',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        lineHeight: '1.6',
        padding: '8px 0',
    },
    '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: '#1f2937',
        borderLeftWidth: '2px',
    },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: '#bfdbfe',
    },
    '.cm-activeLine': {
        backgroundColor: 'rgba(37, 99, 235, 0.04)',
    },
    '.cm-gutters': {
        backgroundColor: '#fafbfc',
        color: '#9ca3af',
        border: 'none',
        borderRight: '1px solid #e5e7eb',
        fontSize: '13px',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    },
    '.cm-activeLineGutter': {
        backgroundColor: 'rgba(37, 99, 235, 0.06)',
        color: '#6b7280',
    },
    '.cm-foldPlaceholder': {
        backgroundColor: '#f3f4f6',
        border: '1px solid #d1d5db',
        color: '#6b7280',
        borderRadius: '3px',
        padding: '0 4px',
    },
    '.cm-searchMatch': {
        backgroundColor: '#fef08a',
        borderRadius: '2px',
    },
    '.cm-searchMatch.cm-searchMatch-selected': {
        backgroundColor: '#fbbf24',
    },
    '.cm-selectionMatch': {
        backgroundColor: 'rgba(37, 99, 235, 0.1)',
    },
    '.cm-matchingBracket': {
        backgroundColor: 'rgba(37, 99, 235, 0.12)',
        outline: '1px solid rgba(37, 99, 235, 0.3)',
    },
    '.cm-panels': {
        backgroundColor: '#f9fafb',
        color: '#1f2937',
        borderBottom: '1px solid #e5e7eb',
    },
    '.cm-tooltip': {
        backgroundColor: '#ffffff',
        border: '1px solid #e5e7eb',
        borderRadius: '6px',
        boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)',
    },
    '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
        backgroundColor: '#2563eb',
        color: '#ffffff',
    },
}, { dark: false });

// ── Dark theme ──────────────────────────────────────────────────
const darkTheme = EditorView.theme({
    '&': {
        backgroundColor: '#1a1b26',
        color: '#d1d5db',
        fontSize: '14px',
    },
    '.cm-content': {
        caretColor: '#aeafad',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
        lineHeight: '1.6',
        padding: '8px 0',
    },
    '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: '#aeafad',
        borderLeftWidth: '2px',
    },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: '#264f78',
    },
    '.cm-activeLine': {
        backgroundColor: 'rgba(255, 255, 255, 0.04)',
    },
    '.cm-gutters': {
        backgroundColor: '#1a1b26',
        color: '#4b5563',
        border: 'none',
        borderRight: '1px solid #2d2f3d',
        fontSize: '13px',
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
    },
    '.cm-activeLineGutter': {
        backgroundColor: 'rgba(255, 255, 255, 0.06)',
        color: '#9ca3af',
    },
    '.cm-foldPlaceholder': {
        backgroundColor: '#2d2f3d',
        border: 'none',
        color: '#6b7280',
    },
    '.cm-searchMatch': {
        backgroundColor: '#72a1ff59',
    },
    '.cm-searchMatch.cm-searchMatch-selected': {
        backgroundColor: '#9aa0a688',
    },
    '.cm-selectionMatch': {
        backgroundColor: '#add6ff26',
    },
    '.cm-matchingBracket, .cm-nonmatchingBracket': {
        backgroundColor: '#bad0f847',
        outline: '1px solid #515a6b',
    },
    '.cm-panels': {
        backgroundColor: '#252526',
        color: '#cccccc',
        borderBottom: '1px solid #3c3c3c',
    },
    '.cm-tooltip': {
        border: '1px solid #454545',
        backgroundColor: '#252526',
        borderRadius: '6px',
    },
    '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
        backgroundColor: '#094771',
        color: '#fff',
    },
}, { dark: true });

// ── Compartments ────────────────────────────────────────────────
const vimCompartment = new Compartment();
const themeCompartment = new Compartment();
const highlightCompartment = new Compartment();

// Create editor configuration
function createExtensions(options = {}) {
    const isDark = options.dark;
    const extensions = [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        history(),
        foldGutter(),
        drawSelection(),
        dropCursor(),
        EditorState.allowMultipleSelections.of(true),
        indentOnInput(),
        // Use our markdown-specific highlight style first, then default as fallback
        highlightCompartment.of([
            syntaxHighlighting(isDark ? markdownHighlightDark : markdownHighlightLight),
            syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        ]),
        bracketMatching(),
        closeBrackets(),
        autocompletion(),
        rectangularSelection(),
        crosshairCursor(),
        highlightSelectionMatches(),
        keymap.of([
            ...closeBracketsKeymap,
            ...defaultKeymap,
            ...searchKeymap,
            ...historyKeymap,
            ...completionKeymap,
            indentWithTab
        ]),
        markdown({ base: markdownLanguage }),
        EditorView.lineWrapping,
        themeCompartment.of(isDark ? darkTheme : lightTheme),
        vimCompartment.of(options.vim ? vim() : []),
    ];
    
    if (options.onUpdate) {
        extensions.push(EditorView.updateListener.of(update => {
            if (update.docChanged) {
                options.onUpdate(update.state.doc.toString());
            }
        }));
    }
    
    return extensions;
}

function createEditor(parent, options = {}) {
    const {
        content = '',
        vim: enableVim = false,
        dark = false,
        onUpdate = null
    } = options;
    
    const state = EditorState.create({
        doc: content,
        extensions: createExtensions({ vim: enableVim, dark, onUpdate })
    });
    
    const view = new EditorView({ state, parent });
    
    return {
        view,
        getContent: () => view.state.doc.toString(),
        setContent: (text) => {
            view.dispatch({
                changes: { from: 0, to: view.state.doc.length, insert: text }
            });
        },
        toggleVim: (enable) => {
            view.dispatch({
                effects: vimCompartment.reconfigure(enable ? vim() : [])
            });
        },
        toggleDark: (enable) => {
            view.dispatch({
                effects: [
                    themeCompartment.reconfigure(enable ? darkTheme : lightTheme),
                    highlightCompartment.reconfigure([
                        syntaxHighlighting(enable ? markdownHighlightDark : markdownHighlightLight),
                        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
                    ]),
                ]
            });
        },
        focus: () => view.focus(),
        destroy: () => view.destroy()
    };
}

export { createEditor, EditorView, EditorState, vim, Vim };
