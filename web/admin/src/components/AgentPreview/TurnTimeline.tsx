import type { PresentationTurn } from "../../types";
import type { AgentPreviewFilters } from "./FilterToolbar";
import { TurnBlock } from "./TurnBlock";

function isCodexProgram(program?: string): boolean {
  return !!program && program.toLowerCase().includes("codex");
}

function countToolRows(rows: { kind: string }[]): number {
  return rows.filter((row) => row.kind === "tool").length;
}

export function buildDisplayTurns(turns: PresentationTurn[], program?: string): PresentationTurn[] {
  if (!isCodexProgram(program)) return turns;

  const display: PresentationTurn[] = [];
  let nextNumber = 1;

  for (const turn of turns) {
    let bufferedActivity: PresentationTurn["rows"] = [];
    let currentRows: PresentationTurn["rows"] | null = null;
    let sawActivityAfterProse = false;

    const pushChunk = (rows: PresentationTurn["rows"], isFinalChunk: boolean) => {
      if (rows.length === 0) return;
      const isRunningChunk = turn.completed_at === null && !turn.interrupted && isFinalChunk;
      display.push({
        ...turn,
        id: `${turn.id}-chunk-${nextNumber}`,
        number: nextNumber,
        completed_at: isRunningChunk ? null : (turn.completed_at ?? turn.started_at),
        interrupted: turn.interrupted && isFinalChunk,
        tool_count: countToolRows(rows),
        rows,
      });
      nextNumber += 1;
    };

    const startResponseChunk = (row: PresentationTurn["rows"][number]) => {
      if (currentRows !== null) {
        pushChunk(currentRows, false);
      }
      currentRows = [row];
      if (bufferedActivity.length > 0) {
        currentRows.push(...bufferedActivity);
        bufferedActivity = [];
      }
      sawActivityAfterProse = false;
    };

    for (const row of turn.rows) {
      if (row.kind === "response") continue;

      if (row.kind === "prose") {
        if (currentRows === null || sawActivityAfterProse) {
          startResponseChunk(row);
        } else {
          currentRows = [...currentRows, row];
        }
        continue;
      }

      if (currentRows === null) {
        bufferedActivity.push(row);
        continue;
      }

      currentRows = [...currentRows, row];
      sawActivityAfterProse = true;
    }

    if (currentRows !== null) {
      pushChunk(currentRows, true);
    } else {
      pushChunk(bufferedActivity, true);
    }
  }

  return display;
}

interface TurnTimelineProps {
  turns: PresentationTurn[];
  program?: string;
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
  program,
  capturedAt,
  snapshotReceivedAt,
  project,
  title,
  filters,
}: TurnTimelineProps) {
  const displayTurns = buildDisplayTurns(turns, program);
  let permissionOffset = 0;

  return (
    <>
      {displayTurns.map((turn) => {
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
