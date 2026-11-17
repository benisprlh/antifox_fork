import type { Task } from "@/lib/types";

// taskLabel renders a concise, identifiable task label used by every task filter
// Findings/dashboard/assets prefer user-set task names, falling back to
// an ID plus truncated description.
export function taskLabel(t: { id: string; name?: string; description?: string }): string {
  const name = (t.name ?? "").trim();
  const desc = (t.description ?? "").trim();
  const body = name || (desc.length > 28 ? desc.slice(0, 28) + "…" : desc) || "Untitled task";
  return `#${t.id} ${body}`; // ID distinguishes tasks; body prefers name, then truncated description.
}

// taskMap builds an id→Task lookup for resolving labels from a task list.
export function taskMap(tasks: Task[]): Map<string, Task> {
  return new Map(tasks.map((t) => [t.id, t]));
}
