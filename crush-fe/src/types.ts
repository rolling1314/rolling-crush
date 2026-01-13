export type FileNode = {
  id: string;
  name: string;
  type: 'file' | 'folder';
  children?: FileNode[];
  content?: string;
  path: string;
  startLine?: number;
  endLine?: number;
};

// Tool call and result types
export interface ToolCall {
  id: string;
  name: string;
  input: string;
  provider_executed?: boolean;
  finished: boolean;
}

export interface ToolResult {
  tool_call_id: string;
  name: string;
  content: string;
  data?: string;
  mime_type?: string;
  metadata?: string;
  is_error: boolean;
}

export interface FinishInfo {
  reason: string;
  time: number;
  message?: string;
  details?: string;
}

// Backend message structure
export interface ContentPart {
  type?: 'text' | 'reasoning' | 'image_url' | 'tool_call' | 'tool_result' | 'finish';
  data?: any;
  // Direct fields (actual backend structure)
  text?: string;
  thinking?: string;
  signature?: string;
  id?: string;
  name?: string;
  input?: string;
  finished?: boolean;
  tool_call_id?: string;
  content?: string;
  is_error?: boolean;
  reason?: string;
  time?: number;
  [key: string]: any;
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

// Permission types
export interface PermissionRequest {
  id: string;
  session_id: string;
  tool_call_id: string;
  tool_name: string;
  action?: string;
}

export type Message = {
  id: string;
  role: 'user' | 'assistant' | 'tool';
  content: string;
  reasoning?: string;
  timestamp: number;
  isStreaming?: boolean;
  toolCalls?: ToolCall[];
  toolResults?: ToolResult[];
  finishInfo?: FinishInfo;
  images?: ImageAttachment[];
};

// Image attachment type
export interface ImageAttachment {
  url: string;
  filename: string;
  mime_type: string;
  size?: number;
  // For local preview before upload
  localPreview?: string;
}

// Image upload response from the server
export interface ImageUploadResponse {
  url: string;
  filename: string;
  mime_type: string;
  size: number;
}

export interface Session {
  id: string;
  project_id?: string;
  title: string;
  message_count: number;
  prompt_tokens: number;
  completion_tokens: number;
  cost: number;
  context_window: number;
  created_at: number;
  updated_at: number;
}
