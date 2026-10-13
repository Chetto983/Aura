import { describe, expect, it } from 'vitest';
import type { ThreadMessageLike } from '@assistant-ui/react';
import type { Approval, ResolveAction } from '../../approvals/useApprovals';
import type { ApprovalResolution } from '../../approvals/useThreadApprovals';
import { refuseDeclinedCall } from '../ExternalStoreChat_folds';

function turn(...ids: readonly string[]): ThreadMessageLike {
  return {
    role: 'assistant',
    content: ids.map((toolCallId) => ({
      type: 'tool-call' as const,
      toolCallId,
      toolName: 'board',
      argsText: '{"action":"delete"}',
    })),
  };
}

function resolution(action: ResolveAction, withheld: string): ApprovalResolution {
  const approval: Approval = {
    token: 'gate-1',
    conversation_id: 'c-1',
    kind: 'approval',
    question: 'Approve board (risk=destructive)?',
    priority: 0,
    withheld_tool_call_id: withheld,
  };
  return { approval, action, attemptId: 'attempt-1' };
}

function refusedIds(messages: readonly ThreadMessageLike[]): string[] {
  return messages.flatMap((message) =>
    typeof message.content === 'string'
      ? []
      : message.content.flatMap((part) =>
          part.type === 'tool-call' && (part as { refused?: boolean }).refused === true
            ? [part.toolCallId ?? '']
            : [],
        ),
  );
}

describe('refuseDeclinedCall', () => {
  it('marks the call a declined approval withheld, and nothing on an accept', () => {
    const messages = [turn('call-a', 'call-b')];
    expect(refusedIds(refuseDeclinedCall(messages, resolution('decline', 'call-b')))).toEqual([
      'call-b',
    ]);
    expect(refuseDeclinedCall(messages, resolution('accept', 'call-b'))).toBe(messages);
  });

  // A blank provider id falls back to a hash of name, arguments and position, so the same call
  // in two turns shares an id (spec 2026-10-12). A reload paints neither of them, and painting
  // the wrong call is worse than painting none, so the live mark follows the same rule.
  it('marks nothing when the withheld id names more than one call in the thread', () => {
    const messages = [turn('call_x'), turn('call-b', 'call_x')];
    expect(refuseDeclinedCall(messages, resolution('decline', 'call_x'))).toBe(messages);
  });
});
