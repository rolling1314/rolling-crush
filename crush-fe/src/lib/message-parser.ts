import { BackendMessage, Message, ToolCall, ToolResult, ContentPart, FinishInfo, ImageAttachment } from '../types';

export const parseBackendMessages = (backendMessages: BackendMessage[]): Message[] => {
  const messages: Message[] = [];
  
  // Create a map of tool calls to associate results later if needed
  // However, in the current structure, tool results come in separate messages with role='tool'
  // or potentially within the same message flow.
  // Based on ccc.json, 'tool' messages are separate.
  
  // We need to merge 'tool' role messages into the previous 'assistant' message that made the call
  // OR keep them separate but ensure the UI handles them. 
  // The ChatPanel expects toolCalls and toolResults to be on the message object.
  // Typically, the assistant message has toolCalls, and we should attach the subsequent tool results to it.

  let lastAssistantMessage: Message | null = null;

  for (const msg of backendMessages) {
    const timestamp = msg.CreatedAt;
    
    if (msg.Role === 'user') {
      // Extract text content
      const content = msg.Parts.find(p => p.text)?.text || '';

      // Extract images
      const images: ImageAttachment[] = msg.Parts
        .filter(p => p.Path && p.MIMEType && p.MIMEType.startsWith('image/'))
        .map(p => {
            // Convert absolute URL to relative for proxy if needed
            // The backend returns absolute URLs like http://localhost:9000/crush-images/...
            // We want to use the proxy at /crush-images/...
            let url = p.Path!;
            if (url.includes('/crush-images/')) {
                const parts = url.split('/crush-images/');
                if (parts.length > 1) {
                    url = `/crush-images/${parts[1]}`;
                }
            }

            const filename = url.split('/').pop() || 'image.png';
            return {
                url: url,
                filename: filename,
                mime_type: p.MIMEType!,
            };
        });

      messages.push({
        id: msg.ID,
        role: 'user',
        content,
        timestamp,
        images: images.length > 0 ? images : undefined
      });
      lastAssistantMessage = null; // Reset
    } else if (msg.Role === 'assistant') {
      const textPart = msg.Parts.find(p => p.text);
      const thinkingPart = msg.Parts.find(p => p.thinking);
      
      // Find all tool calls in this message
      const toolCalls: ToolCall[] = msg.Parts
        .filter(p => p.id && p.name && (p.input !== undefined))
        .map(p => ({
          id: p.id!,
          name: p.name!,
          input: p.input!,
          provider_executed: p.provider_executed,
          finished: p.finished !== false // default to true if not specified
        }));

      const finishPart = msg.Parts.find(p => p.reason);
      const finishInfo: FinishInfo | undefined = finishPart ? {
        reason: finishPart.reason!,
        time: finishPart.time || 0,
        message: finishPart.message
      } : undefined;

      const newMessage: Message = {
        id: msg.ID,
        role: 'assistant',
        content: textPart?.text || '',
        reasoning: thinkingPart?.thinking,
        timestamp,
        toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
        toolResults: [], // Initialize empty, will fill from subsequent tool messages
        finishInfo
      };

      messages.push(newMessage);
      lastAssistantMessage = newMessage;
    } else if (msg.Role === 'tool') {
      // This is a tool result message
      // In this format, Parts contains tool results
      // We need to attach these results to the *last assistant message* that had tool calls
      // matching these IDs.
      
      const results: ToolResult[] = msg.Parts
        .filter(p => p.tool_call_id && p.content)
        .map(p => ({
            tool_call_id: p.tool_call_id!,
            name: p.name || 'unknown',
            content: p.content!,
            data: p.data,
            mime_type: p.mime_type,
            metadata: p.metadata,
            is_error: p.is_error || false
        }));

      if (lastAssistantMessage && lastAssistantMessage.toolCalls) {
        // Attach to the last assistant message
        if (!lastAssistantMessage.toolResults) {
            lastAssistantMessage.toolResults = [];
        }
        lastAssistantMessage.toolResults.push(...results);
      } else {
        // If we can't find a parent, maybe we should add it as a standalone message?
        // But ChatPanel seems to render tool results inside the message bubble.
        // For now, let's assume strict ordering or we could search backwards.
        
        // Fallback: search backwards for the matching tool call
        const matchingMsg = messages.slice().reverse().find(m => 
            m.role === 'assistant' && 
            m.toolCalls?.some(tc => results.some(r => r.tool_call_id === tc.id))
        );

        if (matchingMsg) {
            if (!matchingMsg.toolResults) matchingMsg.toolResults = [];
            matchingMsg.toolResults.push(...results);
        }
      }
    }
  }

  return messages;
};

