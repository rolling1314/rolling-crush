export type FileNode = {
  id: string;
  name: string;
  type: 'file' | 'folder';
  children?: FileNode[];
  content?: string;
  path: string;
};

// Backend message structure
export interface ContentPart {
  type: 'text' | 'reasoning' | 'image_url' | 'tool_call' | 'tool_result';
  data: {
    text?: string;
    thinking?: string;
    signature?: string;
    [key: string]: any;
  };
}

export interface BackendMessage {
  ID: string;
  Role: 'user' | 'assistant' | 'tool';
  SessionID: string;
  Parts: ContentPart[];
  Model?: string;
  Provider?: string;
  CreatedAt: number;
  UpdatedAt: number;
  IsSummaryMessage?: boolean;
}

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  reasoning?: string;
  timestamp: number;
  isStreaming?: boolean;
};

