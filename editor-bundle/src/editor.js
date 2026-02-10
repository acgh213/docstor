/**
 * Docstor CodeMirror 6 Editor Bundle
 * 
 * This bundles all CodeMirror 6 dependencies into a single IIFE
 * that can be loaded in the browser without ES module issues.
 */

import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, highlightActiveLine, highlightActiveLineGutter, drawSelection, rectangularSelection, crosshairCursor, dropCursor } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands';
import { markdown, markdownLanguage } from '@codemirror/lang-markdown';
import { syntaxHighlighting, defaultHighlightStyle, bracketMatching, foldGutter, indentOnInput } from '@codemirror/language';
import { searchKeymap, highlightSelectionMatches } from '@codemirror/search';
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete';
import { vim, Vim } from '@replit/codemirror-vim';

// Theme - a simple dark theme
const darkTheme = EditorView.theme({
    '&': {
        backgroundColor: '#1e1e1e',
        color: '#d4d4d4'
    },
    '.cm-content': {
        caretColor: '#aeafad'
    },
    '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: '#aeafad'
    },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: '#264f78'
    },
    '.cm-panels': {
        backgroundColor: '#252526',
        color: '#cccccc'
    },
    '.cm-panels.cm-panels-top': {
        borderBottom: '1px solid #3c3c3c'
    },
    '.cm-searchMatch': {
        backgroundColor: '#72a1ff59'
    },
    '.cm-searchMatch.cm-searchMatch-selected': {
        backgroundColor: '#9aa0a688'
    },
    '.cm-activeLine': {
        backgroundColor: '#2c2c2c'
    },
    '.cm-selectionMatch': {
        backgroundColor: '#add6ff26'
    },
    '.cm-matchingBracket, .cm-nonmatchingBracket': {
        backgroundColor: '#bad0f847',
        outline: '1px solid #515a6b'
    },
    '.cm-gutters': {
        backgroundColor: '#1e1e1e',
        color: '#858585',
        border: 'none'
    },
    '.cm-activeLineGutter': {
        backgroundColor: '#2c2c2c'
    },
    '.cm-foldPlaceholder': {
        backgroundColor: 'transparent',
        border: 'none',
        color: '#ddd'
    },
    '.cm-tooltip': {
        border: '1px solid #454545',
        backgroundColor: '#252526'
    },
    '.cm-tooltip .cm-tooltip-arrow:before': {
        borderTopColor: 'transparent',
        borderBottomColor: 'transparent'
    },
    '.cm-tooltip .cm-tooltip-arrow:after': {
        borderTopColor: '#252526',
        borderBottomColor: '#252526'
    },
    '.cm-tooltip-autocomplete': {
        '& > ul > li[aria-selected]': {
            backgroundColor: '#094771',
            color: '#fff'
        }
    }
}, { dark: true });

// Light theme (default)
const lightTheme = EditorView.theme({
    '&': {
        backgroundColor: '#ffffff',
        color: '#24292e'
    },
    '.cm-content': {
        caretColor: '#24292e'
    },
    '.cm-cursor, .cm-dropCursor': {
        borderLeftColor: '#24292e'
    },
    '&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection': {
        backgroundColor: '#b3d7ff'
    },
    '.cm-activeLine': {
        backgroundColor: '#f6f8fa'
    },
    '.cm-gutters': {
        backgroundColor: '#f6f8fa',
        color: '#6e7781',
        border: 'none',
        borderRight: '1px solid #d0d7de'
    },
    '.cm-activeLineGutter': {
        backgroundColor: '#eaeef2'
    }
}, { dark: false });

// Compartment for vim mode (can be toggled)
const vimCompartment = new Compartment();
const themeCompartment = new Compartment();

// Create editor configuration
function createExtensions(options = {}) {
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
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
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
        themeCompartment.of(options.dark ? darkTheme : lightTheme),
        vimCompartment.of(options.vim ? vim() : [])
    ];
    
    // Add update listener if provided
    if (options.onUpdate) {
        extensions.push(EditorView.updateListener.of(update => {
            if (update.docChanged) {
                options.onUpdate(update.state.doc.toString());
            }
        }));
    }
    
    return extensions;
}

// Create an editor instance
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
    
    const view = new EditorView({
        state,
        parent
    });
    
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
                effects: themeCompartment.reconfigure(enable ? darkTheme : lightTheme)
            });
        },
        focus: () => view.focus(),
        destroy: () => view.destroy()
    };
}

// Export for global use
export { createEditor, EditorView, EditorState, vim, Vim };
