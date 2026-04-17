import type { PresentationTurn } from "../../types";
import { TurnBlock } from "./TurnBlock";

interface TurnTimelineProps {
  turns: PresentationTurn[];
  capturedAt: Date;
  snapshotReceivedAt: number;
}

/**
 * Renders all turns in chronological order as strict turn blocks.
 * Each turn is independent — no chat-bubble grouping.
 */
export function TurnTimeline({ turns, capturedAt, snapshotReceivedAt }: TurnTimelineProps) {
  return (
    <>
      {turns.map((turn) => (
        <TurnBlock
          key={turn.id}
          turn={turn}
          capturedAt={capturedAt}
          snapshotReceivedAt={snapshotReceivedAt}
        />
      ))}
    </>
  );
}
