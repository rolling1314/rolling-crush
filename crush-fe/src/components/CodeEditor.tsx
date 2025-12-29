import React from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { javascript } from '@codemirror/lang-javascript';
import { vscodeDark } from '@uiw/codemirror-theme-vscode';

interface CodeEditorProps {
  code: string;
  onChange?: (value: string) => void;
  language?: string;
  readOnly?: boolean;
}

export const CodeEditor = ({ code, onChange, readOnly = true }: CodeEditorProps) => {
  return (
    <div className="h-full w-full overflow-hidden bg-[#1e1e1e]">
      <CodeMirror
        value={code}
        height="100%"
        theme={vscodeDark}
        extensions={[javascript({ jsx: true, typescript: true })]}
        onChange={onChange}
        readOnly={readOnly}
        className="h-full text-sm"
      />
    </div>
  );
};

