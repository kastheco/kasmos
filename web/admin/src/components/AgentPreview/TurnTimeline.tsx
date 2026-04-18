import type { PresentationTurn } from "../../types";
import type { AgentPreviewFilters } from "./FilterToolbar";
import { TurnBlock } from "./TurnBlock";

interface TurnTimelineProps {
  turns: PresentationTurn[];
  capturedAt: Date;
  snapshotReceivedAt: number;
  project: string;
  title: string;
  filters: AgentPreviewFilters;
}

/**
 * Renders all turns in chronological order as strict turn blocks.
 * Each turn is independent — no chat-bubble grouping.
 * Computes the cumulative permissionOffset per turn so the first unresolved
 * permission card globally is the only interactive one.
 */
export function TurnTimeline({
  turns,
  capturedAt,
  snapshotReceivedAt,
  project,
  title,
  filters,
}: TurnTimelineProps) {
  let permissionOffset = 0;

  return (
    <>
      {turns.map((turn) => {
        const offset = permissionOffset;
        permissionOffset += turn.rows.filter((r) => r.kind === "permission").length;

        return (
          <TurnBlock
            key={turn.id}
            turn={turn}
            capturedAt={capturedAt}
            snapshotReceivedAt={snapshotReceivedAt}
            project={project}
            title={title}
            filters={filters}
            permissionOffset={offset}
          />
        );
      })}
    </>
  );
}
