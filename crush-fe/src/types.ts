export type FileNode = {
  id: string;
  name: string;
  type: 'file' | 'folder';
  children?: FileNode[];
  content?: string;
  path: string;
};

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: number;
};

