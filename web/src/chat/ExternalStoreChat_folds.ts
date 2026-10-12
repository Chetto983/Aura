import type { AppendMessage, ThreadMessageLike } from '@assistant-ui/react';
import type { ApprovalResolution } from '../approvals/useThreadApprovals';
import type { Asset } from './attachments/types';

// Pure message/asset-fold helpers for the ExternalStoreChat lane (extracted so the component
// file stays under the 600-LOC cap — refactor-on-touch). NO React, NO runtime coupling: every
// function here maps a message list + assets to a new message list, unit-tested in isolation.

export function appendMessageText(message: AppendMessage): string {
  return message.content
    .filter((p): p is { type: 'text'; text: string } => p.type === 'text')
    .map((p) => p.text)
    .join('');
}

export function userMessage(text: string, attachments: readonly Asset[] = []): ThreadMessageLike {
  return {
    id: crypto.randomUUID(),
    role: 'user',
    content: [{ type: 'text', text }],
    ...(attachments.length > 0 ? { metadata: { custom: { attachments } } } : {}),
  };
}

function messageAttachments(message: ThreadMessageLike): readonly Asset[] {
  const metadata = message.metadata?.custom as { attachments?: readonly Asset[] } | undefined;
  return metadata?.attachments ?? [];
}

function withMessageAttachments(
  message: ThreadMessageLike,
  attachments: readonly Asset[],
): ThreadMessageLike {
  const custom = { ...(message.metadata?.custom ?? {}), attachments };
  return { ...message, metadata: { ...message.metadata, custom } };
}

/** The ids a user turn declares it was sent with (migration 0116). */
function declaredAttachmentIDs(message: ThreadMessageLike): readonly string[] {
  const custom = message.metadata?.custom as { attachmentIds?: readonly string[] } | undefined;
  return custom?.attachmentIds ?? [];
}

/**
 * Attach each user turn's assets, by the ids the turn itself declares.
 *
 * An asset carries no message key, so before migration 0116 the only rule was position: the
 * Nth asset went to the Nth user turn, and an image sent with the third message rendered
 * against the first (measured 2026-09-03). A turn now names what it was sent with and gets
 * exactly that; an asset no turn names is placed nowhere. An id with no matching asset is
 * dropped rather than rendered as a placeholder: the asset may have been deleted, and a card
 * for bytes that no longer exist is worse than no card.
 *
 * Agent files never pass through here: the snapshot puts each back on the send_file call that
 * delivered it (migration 0126).
 */
export function attachAssetsToUserMessages(
  messages: readonly ThreadMessageLike[],
  assets: readonly Asset[],
): ThreadMessageLike[] {
  const byID = new Map(assets.map((asset) => [asset.id, asset]));
  return messages.map((message) => {
    const declared = declaredAttachmentIDs(message);
    if (message.role !== 'user' || declared.length === 0) return message;
    const found = declared.flatMap((id) => {
      const asset = byID.get(id);
      if (asset === undefined || asset.status === 'deleted' || asset.status === 'canceled') {
        return [];
      }
      return [asset];
    });
    if (found.length === 0) return message;
    return withMessageAttachments(message, [...messageAttachments(message), ...found]);
  });
}

export function replaceAssetInMessages(
  messages: readonly ThreadMessageLike[],
  asset: Asset,
): ThreadMessageLike[] {
  return messages.map((message) => {
    const attachments = messageAttachments(message);
    if (!attachments.some((item) => item.id === asset.id)) return message;
    return withMessageAttachments(
      message,
      attachments.map((item) => (item.id === asset.id ? asset : item)),
    );
  });
}

/**
 * True when an assistant message carries at least one non-empty TEXT part — a
 * real answer. Messages whose parts are exclusively machinery (tool-call /
 * reasoning / display) must NOT surface the Copy/Regenerate/TTS action bar
 * (operator directive: no message actions on tool cards or reasoning-only
 * turns). Works on both live-built and snapshot-rehydrated messages: the
 * runtime's converted message keeps the same content-part contract.
 */
export function hasAnswerText(message: ThreadMessageLike): boolean {
  if (typeof message.content === 'string') return message.content.trim().length > 0;
  return message.content.some(
    (part) => part.type === 'text' && typeof part.text === 'string' && part.text.trim().length > 0,
  );
}

/** The list with `assistant` in its turn's slot, or appended when the slot is gone. */
export function placeAssistant(
  messages: readonly ThreadMessageLike[],
  index: number,
  assistant: ThreadMessageLike,
): ThreadMessageLike[] {
  const next = messages.slice();
  if (index >= 0 && index < next.length) {
    next[index] = assistant;
  } else {
    next.push(assistant);
  }
  return next;
}

/**
 * The operator's rejection is a refusal too (spec 2026-10-12): once a gateway approval is
 * declined, the call it withheld shows Refused, before any reload. Any other resolution, or an
 * approval that names no call, leaves the list as it is.
 */
export function refuseDeclinedCall(
  messages: ThreadMessageLike[],
  resolution: ApprovalResolution,
): ThreadMessageLike[] {
  const withheld = resolution.approval.withheld_tool_call_id;
  if (resolution.action !== 'decline' || withheld === undefined || withheld === '') {
    return messages;
  }
  const isWithheld = (part: { readonly type: string; readonly toolCallId?: string }) =>
    part.type === 'tool-call' && part.toolCallId === withheld;
  return messages.map((message) =>
    typeof message.content === 'string' || !message.content.some(isWithheld)
      ? message
      : {
          ...message,
          content: message.content.map((part) =>
            isWithheld(part) ? { ...part, isError: true, refused: true } : part,
          ),
        },
  );
}

export function assistantErrorMessage(text: string): ThreadMessageLike {
  return {
    id: crypto.randomUUID(),
    role: 'assistant',
    content: [{ type: 'text', text }],
    status: { type: 'incomplete', reason: 'error' },
  };
}

/** A caller-side abort, which is a cancellation rather than a failure: it must never be
 * folded into the transcript as an assistant error turn. */
export const isAbortError = (error: unknown) =>
  error instanceof DOMException && error.name === 'AbortError';

export function isAbortSignalAborted(signal: AbortSignal): boolean {
  return signal.aborted;
}
