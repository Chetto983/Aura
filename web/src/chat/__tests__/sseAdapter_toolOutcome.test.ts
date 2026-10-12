import { describe, expect, it } from 'vitest';
import {
  messageParts,
  newAssistantTurn,
  reduceFrame,
  toThreadMessage,
  type AguiFrame,
} from '../sseAdapter';
import { toolStatus } from '../toolStatus';

// aura.tool_outcome (spec 2026-10-12, finding 4): TOOL_CALL_RESULT carries no error field,
// so every failed call streamed green until a reload. The translator now names a failed or
// refused call right behind its result, and the reducer marks the tool part with it.

function settledCall(toolCallId: string): readonly AguiFrame[] {
  return [
    { type: 'TOOL_CALL_START', toolCallId, toolCallName: 'web_search' },
    { type: 'TOOL_CALL_END', toolCallId },
    { type: 'TOOL_CALL_RESULT', toolCallId, content: 'error: tool dispatch denied by policy' },
  ];
}

const outcome = (toolCallId: string, value: unknown): AguiFrame => ({
  type: 'CUSTOM',
  name: 'aura.tool_outcome',
  value: { tool_call_id: toolCallId, outcome: value },
});

function toolPart(frames: readonly AguiFrame[]) {
  const state = newAssistantTurn('a1');
  for (const frame of frames) reduceFrame(state, frame);
  const part = messageParts(toThreadMessage(state)).find((p) => p.type === 'tool-call');
  if (part?.type !== 'tool-call') throw new Error('tool part missing');
  return part;
}

describe('sseAdapter — CUSTOM/aura.tool_outcome', () => {
  it('marks a failed call as an error while it streams', () => {
    const part = toolPart([...settledCall('t1'), outcome('t1', 'error')]);
    expect(part).toMatchObject({ isError: true, result: 'error: tool dispatch denied by policy' });
    expect(part.refused).toBeUndefined();
    expect(toolStatus(part)).toBe('error');
  });

  it('marks a refused call as refused, and still as not succeeded for every isError reader', () => {
    const part = toolPart([...settledCall('t1'), outcome('t1', 'refused')]);
    expect(part).toMatchObject({ isError: true, refused: true });
    expect(toolStatus(part)).toBe('refused');
  });

  it('leaves a call without an outcome, an unknown outcome or a malformed payload alone', () => {
    for (const extra of [
      [],
      [outcome('t1', 'exploded')],
      [{ type: 'CUSTOM', name: 'aura.tool_outcome', value: { outcome: 'error' } } as AguiFrame],
    ]) {
      const part = toolPart([...settledCall('t1'), ...extra]);
      expect(part.isError).toBeUndefined();
      expect(toolStatus(part)).toBe('done');
    }
  });
});
