import CodeMirror, { EditorView } from '@uiw/react-codemirror';
import { javascript } from '@codemirror/lang-javascript';
import { vscodeDark } from '@uiw/codemirror-theme-vscode';
import { useMemo, useRef } from 'react';

interface CodeEditorProps {
  code: string;
  onChange?: (value: string) => void;
  language?: string;
  readOnly?: boolean;
  fileName?: string;
  filePath?: string;
}

export const CodeEditor = ({ code, onChange, readOnly = true, fileName, filePath }: CodeEditorProps) => {
  // Use a ref to hold the current file info, so the event handler always accesses the latest values
  // without needing to re-create the extensions (which can cause editor refresh issues)
  const fileInfoRef = useRef({ fileName, filePath });
  fileInfoRef.current = { fileName, filePath };

  const extensions = useMemo(() => {
    return [
      javascript({ jsx: true, typescript: true }),
      EditorView.domEventHandlers({
        copy: (event: ClipboardEvent, view) => {
          const selection = view.state.selection.main;
          if (selection.empty) return;

          const text = view.state.sliceDoc(selection.from, selection.to);
          const startLine = view.state.doc.lineAt(selection.from).number;
          const endLine = view.state.doc.lineAt(selection.to).number;

          const { fileName, filePath } = fileInfoRef.current;
          
          // Debugging
          console.log('Copy intercepted:', { fileName, filePath, startLine, endLine });

          const derivedName = fileName || (filePath ? filePath.split('/').pop() : 'snippet');

          const data = {
            id: `code-${Date.now()}`,
            name: derivedName,
            path: filePath || '',
            type: 'file',
            content: text,
            startLine,
            endLine
          };

          event.clipboardData?.setData('text/plain', text);
          event.clipboardData?.setData('application/json', JSON.stringify(data));
          event.preventDefault();
        }
      })
    ];
  }, []); // Empty dependency array - extensions created once

  return (
    <div className="h-full w-full overflow-hidden bg-[#1e1e1e]">
      <CodeMirror
        value={code}
        height="100%"
        theme={vscodeDark}
        extensions={extensions}
        onChange={onChange}
        readOnly={readOnly}
        className="h-full text-sm"
      />
    </div>
  );
};
